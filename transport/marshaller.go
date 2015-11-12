package transport

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

var Encode = struct {
	Default int16
	JSON    int16
	Gob     int16
}{
	0, 1, 2,
}

type _mshFnOpt struct {
	task, report int16
}

// requirement:
// - each function should be thread-safe
type Marshaller interface {
	Prepare(name string, fn interface{}) (err error)
	EncodeTask(task *Task) (b []byte, err error)
	DecodeTask(b []byte) (task *Task, err error)
	EncodeReport(report *Report) (b []byte, err error)
	DecodeReport(b []byte) (report *Report, err error)
}

//
// container of Marshaller
//
type Marshallers struct {
	msLock     sync.Mutex
	ms         atomic.Value
	fn2mshLock sync.Mutex
	fn2msh     atomic.Value
}

func NewMarshallers() (m *Marshallers) {
	m = &Marshallers{}
	ms := make(map[int16]Marshaller)

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

func (me *Marshallers) Add(id int16, m Marshaller) (err error) {
	me.msLock.Lock()
	defer me.msLock.Unlock()

	ms := me.ms.Load().(map[int16]Marshaller)
	if v, ok := ms[id]; ok {
		err = errors.New(fmt.Sprintf("id %v already exists %v", id, v))
		return
	}

	nms := make(map[int16]Marshaller)
	for k := range ms {
		nms[k] = ms[k]
	}
	nms[id] = m
	me.ms.Store(nms)
	return
}

func (me *Marshallers) Exists(id int16) (exists bool) {
	m := me.ms.Load().(map[int16]Marshaller)
	_, exists = m[id]
	return
}

func (me *Marshallers) Register(name string, fn interface{}, msTask, msReport int16) (err error) {
	me.fn2mshLock.Lock()
	defer me.fn2mshLock.Unlock()

	fns := me.fn2msh.Load().(map[string]*_mshFnOpt)
	if _, ok := fns[name]; ok {
		err = errors.New(fmt.Sprintf("name %v already exists", name))
		return
	}

	// TODO: test case
	// check existence of those id
	m := me.ms.Load().(map[int16]Marshaller)
	chk := func(id int16) (err error) {
		if v, ok := m[id]; ok {
			err = v.Prepare(name, fn)
			if err != nil {
				return
			}
		} else {
			err = errors.New(fmt.Sprintf("id:%v is not registered", id))
			return
		}
		return
	}
	err = chk(msTask)
	if err != nil {
		return
	}
	err = chk(msReport)
	if err != nil {
		return
	}

	// insert the newly create record
	nfns := make(map[string]*_mshFnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name] = &_mshFnOpt{
		task:   msTask,
		report: msReport,
	}
	me.fn2msh.Store(nfns)
	return
}

func (me *Marshallers) EncodeTask(task *Task) (b []byte, err error) {
	// looking for marshaller-option
	fn := me.fn2msh.Load().(map[string]*_mshFnOpt)
	opt, ok := fn[task.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", task))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
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
	b = append(EncodeHeader(task.ID(), opt.task), body...)
	return
}

func (me *Marshallers) DecodeTask(b []byte) (task *Task, err error) {
	h, err := DecodeHeader(b)
	if err != nil {
		return
	}

	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[h.MashID()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", h))
		return
	}

	task, err = m.DecodeTask(b[h.Length():])
	return
}

func (me *Marshallers) EncodeReport(report *Report) (b []byte, err error) {
	// looking for marshaller-option
	fn := me.fn2msh.Load().(map[string]*_mshFnOpt)
	opt, ok := fn[report.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", report))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", report, opt))
		return
	}

	body, err := m.EncodeReport(report)
	if err != nil {
		return
	}
	b = append(EncodeHeader(report.ID(), opt.report), body...)
	return
}

func (me *Marshallers) DecodeReport(b []byte) (report *Report, err error) {
	h, err := DecodeHeader(b)
	if err != nil {
		return
	}

	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[h.MashID()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", h))
		return
	}

	report, err = m.DecodeReport(b[h.Length():])
	return
}

//
// JSON
//

type jsonMarshaller struct {
}

func (me *jsonMarshaller) Prepare(string, interface{}) (err error) {
	return
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

// Gob needs to register type before encode/decode
func (me *gobMarshaller) Prepare(name string, fn interface{}) (err error) {
	fT := reflect.TypeOf(fn)
	if fT.Kind() != reflect.Func {
		err = errors.New(fmt.Sprintf("fn is not a function but %v", fn))
		return
	}

	reg := func(v reflect.Value) (err error) {
		if !v.CanInterface() {
			err = errors.New(fmt.Sprintf("Can't convert to value in input of %v for name:%v", fn, name))
			return
		}

		gob.Register(v.Interface())
		return
	}

	for i := 0; i < fT.NumIn(); i++ {
		// create a zero value of the type of parameters
		err = reg(reflect.Zero(fT.In(i)))
		if err != nil {
			return
		}
	}

	for i := 0; i < fT.NumOut(); i++ {
		err = reg(reflect.Zero(fT.Out(i)))
		if err != nil {
			return
		}
	}

	return
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
