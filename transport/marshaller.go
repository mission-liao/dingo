package transport

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var Encode = struct {
	Default int
	JSON    int
	Gob     int
}{
	0, 1, 2,
}

type _mshFnOpt struct {
	task, report int
}

// requirement:
// - each function should be thread-safe
type Marshaller interface {
	EncodeTask(task *Task) (b []byte, err error)
	DecodeTask(b []byte) (task *Task, err error)
	EncodeReport(report *Report) (b []byte, err error)
	DecodeReport(b []byte) (report *Report, err error)
}

//
// container of Marshaller
//
type marshallers struct {
	msLock     sync.Mutex
	ms         atomic.Value
	fn2mshLock sync.Mutex
	fn2msh     atomic.Value
}

func NewMarshallers() (m *marshallers, err error) {
	m = &marshallers{}
	ms := make(map[int]Marshaller)

	// init marshaller map
	m.msLock.Lock()
	defer m.msLock.Unlock()
	ms[Encode.JSON] = &jsonMarshaller{}
	ms[Encode.Gob] = &gobMarshaller{}
	ms[Encode.Default] = ms[Encode.JSON]
	m.ms.Store(ms)

	// init map from function to marshaller
	m.fn2mshLock.Lock()
	defer m.fn2mshLock.Unlock()
	m.fn2msh.Store(make(map[string]*_mshFnOpt))

	return
}

func (me *marshallers) Add(id int, m Marshaller) (err error) {
	me.msLock.Lock()
	defer me.msLock.Unlock()

	ms := me.ms.Load().(map[int]Marshaller)
	if v, ok := ms[id]; ok {
		err = errors.New(fmt.Sprintf("id %v already exists %v", id, v))
		return
	}

	nms := make(map[int]Marshaller)
	for k := range ms {
		nms[k] = ms[k]
	}
	nms[id] = m
	me.ms.Store(nms)
	return
}

func (me *marshallers) Exists(id int) (exists bool) {
	m := me.ms.Load().(map[int]Marshaller)
	_, exists = m[id]
	return
}

func (me *marshallers) Register(name string, msTask, msReport int) (err error) {
	me.fn2mshLock.Lock()
	defer me.fn2mshLock.Unlock()

	m := me.fn2msh.Load().(map[string]*_mshFnOpt)
	if _, ok := m[name]; ok {
		err = errors.New(fmt.Sprintf("name %v already exists", name))
		return
	}

	nm := make(map[string]*_mshFnOpt)
	for k := range m {
		nm[k] = m[k]
	}
	nm[name] = &_mshFnOpt{
		task:   msTask,
		report: msReport,
	}
	me.fn2msh.Store(nm)
	return
}

func (me *marshallers) EncodeTask(task *Task) (b []byte, err error) {
	// looking for marshaller-option
	fn := me.fn2msh.Load().(map[string]*_mshFnOpt)
	opt, ok := fn[task.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", task))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", task, opt))
		return
	}

	body, err := m.EncodeTask(task)
	if err != nil {
		return
	}

	// put a head to record which marshaller we use
	head := make([]byte, 8)
	binary.PutVarint(head, int64(opt.task))
	b = append(head, body...)
	return
}

func (me *marshallers) DecodeTask(b []byte) (task *Task, err error) {
	ms := me.ms.Load().(map[int]Marshaller)
	head, body := b[:8], b[8:]
	which, err := binary.ReadVarint(bytes.NewBuffer(head))
	if err != nil {
		return
	}

	m, ok := ms[int(which)]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", which))
		return
	}

	task, err = m.DecodeTask(body)
	return
}

func (me *marshallers) EncodeReport(report *Report) (b []byte, err error) {
	// looking for marshaller-option
	fn := me.fn2msh.Load().(map[string]*_mshFnOpt)
	opt, ok := fn[report.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", report))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", report, opt))
		return
	}

	body, err := m.EncodeReport(report)
	if err != nil {
		return
	}
	head := make([]byte, 8)
	binary.PutVarint(head, int64(opt.report))
	b = append(head, body...)
	return
}

func (me *marshallers) DecodeReport(b []byte) (report *Report, err error) {
	ms := me.ms.Load().(map[int]Marshaller)
	head, body := b[:8], b[8:]
	which, err := binary.ReadVarint(bytes.NewBuffer(head))
	if err != nil {
		return
	}
	m, ok := ms[int(which)]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", which))
		return
	}

	report, err = m.DecodeReport(body)
	return
}

//
// JSON
//

type jsonMarshaller struct {
}

func (me *jsonMarshaller) EncodeTask(task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	b, err = json.Marshal(task)
	return
}

func (me *jsonMarshaller) DecodeTask(b []byte) (task *Task, err error) {
	var t Task
	err = json.Unmarshal(b, &t)
	if err == nil {
		task = &t
	}
	return
}

func (me *jsonMarshaller) EncodeReport(report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	b, err = json.Marshal(report)
	return
}

func (me *jsonMarshaller) DecodeReport(b []byte) (report *Report, err error) {
	var r Report
	err = json.Unmarshal(b, &r)
	if err == nil {
		report = &r
	}
	return
}

//
// Gob
//

type gobMarshaller struct {
}

func (me *gobMarshaller) EncodeTask(task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	var buff *bytes.Buffer = new(bytes.Buffer)

	enc := gob.NewEncoder(buff)
	err = enc.Encode(task)
	if err == nil {
		b = buff.Bytes()
	}
	return
}

func (me *gobMarshaller) DecodeTask(b []byte) (task *Task, err error) {
	var (
		buff *bytes.Buffer = bytes.NewBuffer(b)
		t    Task
	)

	dec := gob.NewDecoder(buff)
	err = dec.Decode(&t)
	if err == nil {
		task = &t
	}
	return
}

func (me *gobMarshaller) EncodeReport(report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}
	var buff *bytes.Buffer = new(bytes.Buffer)

	enc := gob.NewEncoder(buff)
	err = enc.Encode(report)
	if err == nil {
		b = buff.Bytes()
	}
	return
}

func (me *gobMarshaller) DecodeReport(b []byte) (report *Report, err error) {
	var (
		buff *bytes.Buffer = bytes.NewBuffer(b)
		r    Report
	)

	dec := gob.NewDecoder(buff)
	err = dec.Decode(&r)
	if err == nil {
		report = &r
	}
	return
}
