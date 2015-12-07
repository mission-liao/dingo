package transport

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

type fnOpt struct {
	fn   interface{}
	mash struct {
		task, report int16
	}
}

type Mgr struct {
	msLock     sync.Mutex
	ms         atomic.Value
	fn2optLock sync.Mutex
	fn2opt     atomic.Value
}

func NewMgr() (c *Mgr) {
	c = &Mgr{}

	// init for marshaller's'
	ms := make(map[int16]Marshaller)
	c.msLock.Lock()
	defer c.msLock.Unlock()
	ms[Encode.JSON] = &struct {
		JsonMarshaller
		GenericInvoker
	}{}
	ms[Encode.GOB] = &struct {
		GobMarshaller
		GenericInvoker
	}{}
	ms[Encode.Default] = ms[Encode.JSON]
	c.ms.Store(ms)

	// init map from name of function to options
	c.fn2optLock.Lock()
	defer c.fn2optLock.Unlock()
	c.fn2opt.Store(make(map[string]*fnOpt))

	return
}

func (me *Mgr) AddMarshaller(id int16, m Marshaller) (err error) {
	// a Marshaller should depend on an Invoker
	_, ok := m.(Invoker)
	if !ok {
		err = errors.New(fmt.Sprintf("should have Invoker interface, %v", m))
		return
	}

	me.msLock.Lock()
	defer me.msLock.Unlock()

	ms := me.ms.Load().(map[int16]Marshaller)
	if v, ok := ms[id]; ok {
		err = errors.New(fmt.Sprintf("marshaller id %v already exists %v", id, v))
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

func (me *Mgr) Register(name string, fn interface{}, msTask, msReport int16) (err error) {
	if uint16(len(name)) >= ^uint16(0) {
		err = errors.New(fmt.Sprintf("length of name exceeds maximum: %v", len(name)))
		return
	}

	// TODO: test case

	// check existence of marshaller IDs
	ms := me.ms.Load().(map[int16]Marshaller)
	chk := func(id int16) (err error) {
		if v, ok := ms[id]; ok {
			err = v.Prepare(name, fn)
			if err != nil {
				return
			}
		} else {
			err = errors.New(fmt.Sprintf("marshaller id:%v is not registered", id))
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

	// insert the newly created record
	me.fn2optLock.Lock()
	defer me.fn2optLock.Unlock()

	fns := me.fn2opt.Load().(map[string]*fnOpt)
	if _, ok := fns[name]; ok {
		err = errors.New(fmt.Sprintf("name %v already exists", name))
		return
	}
	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name] = &fnOpt{
		fn: fn,
		mash: struct {
			task, report int16
		}{msTask, msReport},
	}
	me.fn2opt.Store(nfns)
	return
}

func (me *Mgr) EncodeTask(task *Task) (b []byte, err error) {
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[task.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", task))
		return
	}

	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.mash.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", task, opt))
		return
	}

	b, err = m.EncodeTask(task)
	return
}

func (me *Mgr) DecodeTask(b []byte) (task *Task, err error) {
	h, err := DecodeHeader(b)
	if err != nil {
		return
	}

	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[h.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", h))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.mash.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", h))
		return
	}

	task, err = m.DecodeTask(h, b)
	return
}

func (me *Mgr) EncodeReport(report *Report) (b []byte, err error) {
	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[report.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", report))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.mash.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", report, opt))
		return
	}

	b, err = m.EncodeReport(report)
	return
}

func (me *Mgr) DecodeReport(b []byte) (report *Report, err error) {
	h, err := DecodeHeader(b)
	if err != nil {
		return
	}

	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[h.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", h))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.mash.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", h))
		return
	}

	report, err = m.DecodeReport(h, b)
	return
}

func (me *Mgr) Call(t *Task) (ret []interface{}, err error) {
	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[t.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", t))
		return
	}

	// looking for the marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.mash.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", t, opt))
		return
	}

	ret, err = m.(Invoker).Call(opt.fn, t.Args())
	return
}

func (me *Mgr) Return(r *Report) (err error) {
	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[r.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", r))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int16]Marshaller)
	m, ok := ms[opt.mash.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", r, opt))
		return
	}

	var ret []interface{}
	ret, err = m.(Invoker).Return(opt.fn, r.Return())
	if err == nil {
		r.SetReturn(ret)
	}
	return
}
