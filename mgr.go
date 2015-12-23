package dingo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

type fnOpt struct {
	fn   interface{}
	mash struct {
		task, report int
	}
	idMaker int
	opt     *Option
}

type mgr struct {
	msLock     sync.Mutex
	ms         atomic.Value
	imsLock    sync.Mutex
	ims        atomic.Value
	fn2optLock sync.Mutex
	fn2opt     atomic.Value
}

func newMgr() (c *mgr) {
	c = &mgr{}

	// init for marshaller's'
	ms := make(map[int]Marshaller)
	c.msLock.Lock()
	defer c.msLock.Unlock()

	// only GenericInvoker can handle things from JsonMarshaller
	ms[Encode.JSON] = &struct {
		JsonMarshaller
		GenericInvoker
	}{}

	// JsonSafeMarshaller is ok with LazyInvoker
	ms[Encode.JSONSAFE] = &struct {
		CustomMarshaller
		LazyInvoker
	}{
		CustomMarshaller{Codec: &JsonSafeCodec{}},
		LazyInvoker{},
	}

	// LazyInvoker 'should' be faster than GenericInvoker
	ms[Encode.GOB] = &struct {
		GobMarshaller
		LazyInvoker
	}{}
	ms[Encode.Default] = ms[Encode.JSONSAFE]

	c.ms.Store(ms)

	// init for id-maker's'
	ims := make(map[int]IDMaker)
	c.imsLock.Lock()
	defer c.imsLock.Unlock()

	ims[ID.UUID] = &uuidMaker{}
	ims[ID.Default] = ims[ID.UUID]
	c.ims.Store(ims)

	// init map from name of function to options
	c.fn2optLock.Lock()
	defer c.fn2optLock.Unlock()
	c.fn2opt.Store(make(map[string]*fnOpt))

	return
}

func (me *mgr) AddIdMaker(id int, m IDMaker) (err error) {
	me.imsLock.Lock()
	defer me.imsLock.Unlock()

	ims := me.ims.Load().(map[int]IDMaker)
	if v, ok := ims[id]; ok {
		err = errors.New(fmt.Sprintf("marshaller id %v already exists %v", id, v))
		return
	}

	nims := make(map[int]IDMaker)
	for k := range ims {
		nims[k] = ims[k]
	}
	nims[id] = m
	me.ims.Store(nims)
	return
}

func (me *mgr) AddMarshaller(id int, m Marshaller) (err error) {
	// a Marshaller should depend on an Invoker
	_, ok := m.(Invoker)
	if !ok {
		err = errors.New(fmt.Sprintf("should have Invoker interface, %v", m))
		return
	}

	me.msLock.Lock()
	defer me.msLock.Unlock()

	ms := me.ms.Load().(map[int]Marshaller)
	if v, ok := ms[id]; ok {
		err = errors.New(fmt.Sprintf("marshaller id %v already exists %v", id, v))
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

func (me *mgr) Register(name string, fn interface{}, msTask, msReport int) (err error) {
	if uint(len(name)) >= ^uint(0) {
		err = errors.New(fmt.Sprintf("length of name exceeds maximum: %v", len(name)))
		return
	}

	// check existence of marshaller IDs
	ms := me.ms.Load().(map[int]Marshaller)
	chk := func(id int) (err error) {
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
		fn:  fn,
		opt: NewOption(),
		mash: struct {
			task, report int
		}{msTask, msReport},
		idMaker: ID.Default,
	}
	me.fn2opt.Store(nfns)
	return
}

func (me *mgr) SetOption(name string, opt *Option) (err error) {
	if opt == nil {
		err = errors.New("nil Option is not acceptable")
		return
	}

	me.fn2optLock.Lock()
	defer me.fn2optLock.Unlock()

	fns := me.fn2opt.Load().(map[string]*fnOpt)
	if _, ok := fns[name]; !ok {
		err = errors.New(fmt.Sprintf("name %v doesn't exists", name))
		return
	}
	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name].opt = opt
	me.fn2opt.Store(nfns)

	return
}

func (me *mgr) SetIDMaker(name string, id int) (err error) {
	// check existence of id
	ims := me.ims.Load().(map[int]IDMaker)
	_, ok := ims[id]
	if !ok {
		err = errors.New(fmt.Sprintf("idMaker not found: %v %v", name, id))
		return
	}

	me.fn2optLock.Lock()
	defer me.fn2optLock.Unlock()

	fns := me.fn2opt.Load().(map[string]*fnOpt)
	if _, ok := fns[name]; !ok {
		err = errors.New(fmt.Sprintf("name %v doesn't exists", name))
		return
	}
	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name].idMaker = id
	me.fn2opt.Store(nfns)

	return
}

func (me *mgr) GetOption(name string) (opt *Option, err error) {
	fns := me.fn2opt.Load().(map[string]*fnOpt)
	if fn, ok := fns[name]; ok {
		opt = fn.opt
	} else {
		err = errors.New(fmt.Sprintf("name %v doesn't exists", name))
	}

	return
}

func (me *mgr) ComposeTask(name string, o *Option, args []interface{}) (t *Task, err error) {
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[name]
	if !ok {
		err = errors.New(fmt.Sprintf("idMaker option not found: %v %v", name, opt))
		return
	}

	ims := me.ims.Load().(map[int]IDMaker)
	m, ok := ims[opt.idMaker]
	if !ok {
		err = errors.New(fmt.Sprintf("idMaker not found: %v %v", name, opt))
		return
	}

	if o == nil {
		o = NewOption()
	}

	id, err := m.NewID()
	if err != nil {
		return
	}

	t = &Task{
		H: NewHeader(id, name),
		P: &TaskPayload{
			O: o,
			A: args,
		},
	}
	return
}

func (me *mgr) EncodeTask(task *Task) (b []byte, err error) {
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[task.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", task))
		return
	}

	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.mash.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", task, opt))
		return
	}

	b, err = m.EncodeTask(opt.fn, task)
	return
}

func (me *mgr) DecodeTask(b []byte) (task *Task, err error) {
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
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.mash.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", h))
		return
	}

	task, err = m.DecodeTask(h, opt.fn, b)
	return
}

func (me *mgr) EncodeReport(report *Report) (b []byte, err error) {
	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[report.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", report))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.mash.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", report, opt))
		return
	}

	b, err = m.EncodeReport(opt.fn, report)
	return
}

func (me *mgr) DecodeReport(b []byte) (report *Report, err error) {
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
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.mash.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v", h))
		return
	}

	report, err = m.DecodeReport(h, opt.fn, b)
	return
}

func (me *mgr) Call(t *Task) (ret []interface{}, err error) {
	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[t.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", t))
		return
	}

	// looking for the marshaller
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.mash.task]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", t, opt))
		return
	}

	ret, err = m.(Invoker).Call(opt.fn, t.Args())
	return
}

func (me *mgr) Return(r *Report) (err error) {
	// looking for marshaller-option
	fn := me.fn2opt.Load().(map[string]*fnOpt)
	opt, ok := fn[r.Name()]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller option not found: %v", r))
		return
	}

	// looking for marshaller
	ms := me.ms.Load().(map[int]Marshaller)
	m, ok := ms[opt.mash.report]
	if !ok {
		err = errors.New(fmt.Sprintf("marshaller not found: %v %v", r, opt))
		return
	}

	var ret []interface{}
	ret, err = m.(Invoker).Return(opt.fn, r.Return())
	if err == nil {
		r.setReturn(ret)
	}
	return
}
