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

/*
 every setting related to worker functions is here.
*/
type fnMgr struct {
	msLock     sync.Mutex
	ms         atomic.Value
	imsLock    sync.Mutex
	ims        atomic.Value
	fn2optLock sync.Mutex
	fn2opt     atomic.Value
}

func newFnMgr() (c *fnMgr) {
	c = &fnMgr{}

	// init for marshaller's'
	ms := make(map[int]Marshaller)
	c.msLock.Lock()
	defer c.msLock.Unlock()

	// only GenericInvoker can handle things from JsonMarshaller
	ms[Encode.JSON] = &struct {
		JsonMarshaller
		GenericInvoker
	}{}

	// JSONSafeMarshaller is ok with LazyInvoker
	ms[Encode.JSONSAFE] = &struct {
		CustomMarshaller
		LazyInvoker
	}{
		CustomMarshaller{Codec: &JSONSafeCodec{}},
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

func (mgr *fnMgr) AddIDMaker(id int, m IDMaker) (err error) {
	mgr.imsLock.Lock()
	defer mgr.imsLock.Unlock()

	ims := mgr.ims.Load().(map[int]IDMaker)
	if v, ok := ims[id]; ok {
		err = fmt.Errorf("marshaller id %v already exists %v", id, v)
		return
	}

	nims := make(map[int]IDMaker)
	for k := range ims {
		nims[k] = ims[k]
	}
	nims[id] = m
	mgr.ims.Store(nims)
	return
}

func (mgr *fnMgr) AddMarshaller(id int, m Marshaller) (err error) {
	// a Marshaller should depend on an Invoker
	_, ok := m.(Invoker)
	if !ok {
		err = fmt.Errorf("should have Invoker interface, %v", m)
		return
	}

	mgr.msLock.Lock()
	defer mgr.msLock.Unlock()

	ms := mgr.ms.Load().(map[int]Marshaller)
	if v, ok := ms[id]; ok {
		err = fmt.Errorf("marshaller id %v already exists %v", id, v)
		return
	}

	nms := make(map[int]Marshaller)
	for k := range ms {
		nms[k] = ms[k]
	}
	nms[id] = m
	mgr.ms.Store(nms)
	return
}

func (mgr *fnMgr) Register(name string, fn interface{}) (err error) {
	if uint(len(name)) >= ^uint(0) {
		err = fmt.Errorf("length of name exceeds maximum: %v", len(name))
		return
	}

	ms := mgr.ms.Load().(map[int]Marshaller)
	if err = ms[Encode.Default].Prepare(name, fn); err != nil {
		return
	}

	// insert the newly created record
	mgr.fn2optLock.Lock()
	defer mgr.fn2optLock.Unlock()

	fns := mgr.fn2opt.Load().(map[string]*fnOpt)
	if _, ok := fns[name]; ok {
		err = fmt.Errorf("name %v already exists", name)
		return
	}
	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name] = &fnOpt{
		fn:  fn,
		opt: DefaultOption(),
		mash: struct {
			task, report int
		}{Encode.Default, Encode.Default},
		idMaker: ID.Default,
	}
	mgr.fn2opt.Store(nfns)
	return
}

func (mgr *fnMgr) SetMarshaller(name string, msTask, msReport int) (err error) {
	mgr.fn2optLock.Lock()
	defer mgr.fn2optLock.Unlock()

	var (
		opt *fnOpt
		ok  bool
	)

	fns := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fns[name]; !ok {
		err = fmt.Errorf("name %v doesn't exists", name)
		return
	}

	// check existence of marshaller IDs
	ms := mgr.ms.Load().(map[int]Marshaller)
	chk := func(id int) (err error) {
		if v, ok := ms[id]; ok {
			if err = v.Prepare(name, opt.fn); err != nil {
				return
			}
		} else {
			err = fmt.Errorf("marshaller id:%v is not registered", id)
			return
		}
		return
	}
	if err = chk(msTask); err != nil {
		return
	}
	if err = chk(msReport); err != nil {
		return
	}

	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name].mash.task = msTask
	nfns[name].mash.report = msReport
	mgr.fn2opt.Store(nfns)

	return
}

func (mgr *fnMgr) SetOption(name string, opt *Option) (err error) {
	if opt == nil {
		err = errors.New("nil Option is not acceptable")
		return
	}

	mgr.fn2optLock.Lock()
	defer mgr.fn2optLock.Unlock()

	fns := mgr.fn2opt.Load().(map[string]*fnOpt)
	if _, ok := fns[name]; !ok {
		err = fmt.Errorf("name %v doesn't exists", name)
		return
	}
	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name].opt = opt
	mgr.fn2opt.Store(nfns)

	return
}

func (mgr *fnMgr) SetIDMaker(name string, id int) (err error) {
	// check existence of id
	ims := mgr.ims.Load().(map[int]IDMaker)
	if _, ok := ims[id]; !ok {
		err = fmt.Errorf("idMaker not found: %v %v", name, id)
		return
	}

	mgr.fn2optLock.Lock()
	defer mgr.fn2optLock.Unlock()

	fns := mgr.fn2opt.Load().(map[string]*fnOpt)
	if _, ok := fns[name]; !ok {
		err = fmt.Errorf("name %v doesn't exists", name)
		return
	}
	nfns := make(map[string]*fnOpt)
	for k := range fns {
		nfns[k] = fns[k]
	}
	nfns[name].idMaker = id
	mgr.fn2opt.Store(nfns)

	return
}

func (mgr *fnMgr) GetOption(name string) (opt *Option, err error) {
	fns := mgr.fn2opt.Load().(map[string]*fnOpt)
	if fn, ok := fns[name]; ok {
		opt = fn.opt
	} else {
		err = fmt.Errorf("name %v doesn't exists", name)
	}

	return
}

func (mgr *fnMgr) ComposeTask(name string, o *Option, args []interface{}) (t *Task, err error) {
	fn := mgr.fn2opt.Load().(map[string]*fnOpt)

	var (
		opt *fnOpt
		m   IDMaker
		ok  bool
		id  string
	)
	if opt, ok = fn[name]; !ok {
		err = fmt.Errorf("idMaker option not found: %v %v", name, opt)
		return
	}

	ims := mgr.ims.Load().(map[int]IDMaker)
	if m, ok = ims[opt.idMaker]; !ok {
		err = fmt.Errorf("idMaker not found: %v %v", name, opt)
		return
	}

	if o == nil {
		o = DefaultOption()
	}

	if id, err = m.NewID(); err != nil {
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

func (mgr *fnMgr) EncodeTask(task *Task) (b []byte, err error) {
	var (
		opt *fnOpt
		m   Marshaller
		ok  bool
	)

	fn := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fn[task.Name()]; !ok {
		err = fmt.Errorf("marshaller option not found: %v", task)
		return
	}

	ms := mgr.ms.Load().(map[int]Marshaller)
	if m, ok = ms[opt.mash.task]; !ok {
		err = fmt.Errorf("marshaller not found: %v %v", task, opt)
		return
	}

	b, err = m.EncodeTask(opt.fn, task)
	return
}

func (mgr *fnMgr) DecodeTask(b []byte) (task *Task, err error) {
	var (
		h   *Header
		m   Marshaller
		opt *fnOpt
		ok  bool
	)
	if h, err = DecodeHeader(b); err != nil {
		return
	}

	// looking for marshaller-option
	fn := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fn[h.Name()]; !ok {
		err = fmt.Errorf("marshaller option not found: %v", h)
		return
	}

	// looking for marshaller
	ms := mgr.ms.Load().(map[int]Marshaller)
	if m, ok = ms[opt.mash.task]; !ok {
		err = fmt.Errorf("marshaller not found: %v", h)
		return
	}

	task, err = m.DecodeTask(h, opt.fn, b)
	return
}

func (mgr *fnMgr) EncodeReport(report *Report) (b []byte, err error) {
	var (
		opt *fnOpt
		m   Marshaller
		ok  bool
	)
	// looking for marshaller-option
	fn := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fn[report.Name()]; !ok {
		err = fmt.Errorf("marshaller option not found: %v", report)
		return
	}

	// looking for marshaller
	ms := mgr.ms.Load().(map[int]Marshaller)
	if m, ok = ms[opt.mash.report]; !ok {
		err = fmt.Errorf("marshaller not found: %v %v", report, opt)
		return
	}

	b, err = m.EncodeReport(opt.fn, report)
	return
}

func (mgr *fnMgr) DecodeReport(b []byte) (report *Report, err error) {
	var (
		h   *Header
		m   Marshaller
		opt *fnOpt
		ok  bool
	)

	if h, err = DecodeHeader(b); err != nil {
		return
	}

	// looking for marshaller-option
	fn := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fn[h.Name()]; !ok {
		err = fmt.Errorf("marshaller option not found: %v", h)
		return
	}

	// looking for marshaller
	ms := mgr.ms.Load().(map[int]Marshaller)
	if m, ok = ms[opt.mash.report]; !ok {
		err = fmt.Errorf("marshaller not found: %v", h)
		return
	}

	report, err = m.DecodeReport(h, opt.fn, b)
	return
}

func (mgr *fnMgr) Call(t *Task) (ret []interface{}, err error) {
	var (
		opt *fnOpt
		m   Marshaller
		ok  bool
	)

	// looking for marshaller-option
	fn := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fn[t.Name()]; !ok {
		err = fmt.Errorf("marshaller option not found: %v", t)
		return
	}

	// looking for the marshaller
	ms := mgr.ms.Load().(map[int]Marshaller)
	if m, ok = ms[opt.mash.task]; !ok {
		err = fmt.Errorf("marshaller not found: %v %v", t, opt)
		return
	}

	ret, err = m.(Invoker).Call(opt.fn, t.Args())
	return
}

func (mgr *fnMgr) Return(r *Report) (err error) {
	var (
		opt *fnOpt
		m   Marshaller
		ok  bool
	)
	// looking for marshaller-option
	fn := mgr.fn2opt.Load().(map[string]*fnOpt)
	if opt, ok = fn[r.Name()]; !ok {
		err = fmt.Errorf("marshaller option not found: %v", r)
		return
	}

	// looking for marshaller
	ms := mgr.ms.Load().(map[int]Marshaller)
	if m, ok = ms[opt.mash.report]; !ok {
		err = fmt.Errorf("marshaller not found: %v %v", r, opt)
		return
	}

	var ret []interface{}
	if ret, err = m.(Invoker).Return(opt.fn, r.Return()); err == nil {
		r.setReturn(ret)
	}
	return
}
