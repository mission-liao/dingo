package dingo

import (
	// standard
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	// internal
	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

type _eventListener struct {
	targets, level int
	events         chan *common.Event
}

type App interface {
	//
	Close() error

	// hire a set of workers for a pattern
	//
	// parameters ->
	// - match: tasks in dingo are recoginized by a 'name', this function decides
	//          which task to accept by returning true.
	// - fn: the function that actually perform the task.
	// - count: count of workers to be initialized.
	//
	// returns ->
	// - id: identifier of this group of workers
	// - remain: remaining count of workers that not initialized.
	// - err: any error produced
	Register(m Matcher, fn interface{}, count int) (id string, remain int, err error)

	// attach an instance, instance could be any instance implementing
	// backend.Reporter, backend.Backend, broker.Producer, broker.Consumer.
	//
	// parameters:
	// - obj: object to be attached
	// - types: interfaces contained in 'obj', refer to dingo.InstT
	// returns:
	// - id: identifier assigned to this object, 0 is invalid value
	// - err: errors
	Use(obj interface{}, types int) (id int, used int, err error)

	// send a task
	//
	Call(name string, opt *meta.Option, args ...interface{}) (<-chan meta.Report, error)

	// get the channel to receive events from 'dingo', based on the id of requested object
	//
	Listen(target, level, expected_id int) (int, <-chan *common.Event, error)

	// release the error channel
	//
	StopListen(id int) error
}

//
// app
//

type _object struct {
	used int
	obj  interface{}
}

type _app struct {
	invoker meta.Invoker

	cfg          Config
	objsLock     sync.RWMutex
	objs         map[int]*_object
	producer     broker.Producer
	consumer     broker.Consumer
	store        backend.Store
	reporter     backend.Reporter
	eventMux     *common.Mux
	eventOut     atomic.Value
	eventOutLock sync.Mutex

	// internal routines
	mappers  *_mappers
	monitors *_monitors
	events   *common.Routines
}

// factory function
//
func NewApp(c Config) (App, error) {
	v := &_app{
		objs:     make(map[int]*_object),
		invoker:  meta.NewDefaultInvoker(),
		cfg:      c,
		eventMux: common.NewMux(),
		events:   common.NewRoutines(),
	}

	// refer to 'ReadMostly' example in sync/atomic
	v.eventOut.Store(make(map[int]*_eventListener))

	// should only initiate 1 routine for errors,
	// or the order of reported errors would not be
	// preserved.
	go v._event_routine_(v.events.New(), v.events.Wait(), v.eventMux.Out())

	remain, err := v.eventMux.More(1)
	if err == nil && remain != 0 {
		err = errors.New(fmt.Sprintf("Unable to allocate mux routine: %v", remain))
	}

	return v, err
}

//
func (me *_app) _event_routine_(quit <-chan int, wait *sync.WaitGroup, events <-chan *common.MuxOut) {
	defer wait.Done()
	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case v, ok := <-events:
			if !ok {
				goto cleanup
			}

			e := v.Value.(*common.Event)
			m := me.eventOut.Load().(map[int]*_eventListener)
			// to the channel containing everythin errors
			for _, eln := range m {
				if (eln.targets&e.Origin) == 0 || eln.level > e.Level {
					continue
				}

				// non-blocking channel sending
				select {
				case eln.events <- e:
				default:
					// drop this event
					// TODO: log it?
				}
			}
		}
	}
cleanup:
}

//
// App interface
//

func (me *_app) Close() (err error) {
	me.objsLock.Lock()
	defer me.objsLock.Unlock()

	chk := func(err_ error) {
		if err == nil {
			err = err_
		}
	}

	// TODO: the right shutdown procedure:
	// - broadcase 'quit' message to 'all' routines
	// - await 'all' routines to finish cleanup
	// right now we would send a quit message to 'one' routine, and wait it done.

	for _, v := range me.objs {
		if v.used&common.InstT.REPORTER == common.InstT.REPORTER {
			chk(me.reporter.Unbind())
		}

		s, ok := v.obj.(common.Object)
		if ok {
			chk(s.Close())
		}
	}

	// shutdown mappers
	if me.mappers != nil {
		chk(me.mappers.Close())
		me.mappers = nil
	}

	// shutdown monitors
	if me.monitors != nil {
		chk(me.monitors.Close())
		me.monitors = nil
	}

	// shutdown the monitor of error channels
	me.events.Close()
	me.eventMux.Close()
	func() {
		me.eventOutLock.Lock()
		defer me.eventOutLock.Unlock()

		m := me.eventOut.Load().(map[int]*_eventListener)
		for _, v := range m {
			// send channel-close event
			close(v.events)
		}
		me.eventOut.Store(make(map[int]*_eventListener))
	}()

	return
}

func (me *_app) Register(m Matcher, fn interface{}, count int) (id string, remain int, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	remain = count

	if me.mappers == nil && me.monitors == nil {
		err = errors.New("no monitors/mappers available.")
		return
	}

	if me.mappers != nil {
		id, remain, err = me.mappers.allocateWorkers(m, fn, count)
		if err != nil {
			return
		}
	}

	// TODO: add test case the makes monitors and mappers sync

	if me.monitors != nil {
		err = me.monitors.register(m, fn)
		if err != nil {
			return
		}
	}
	return
}

func (me *_app) Use(obj interface{}, types int) (id int, used int, err error) {
	me.objsLock.Lock()
	defer me.objsLock.Unlock()

	var (
		producer broker.Producer
		consumer broker.Consumer
		store    backend.Store
		reporter backend.Reporter
		mns      *_monitors
		mps      *_mappers
		ok       bool
		eids     []int = []int{}
	)

	defer func() {
		if err == nil {
			return
		}

		// clean up
		for _, v := range eids {
			_, err_ := me.eventMux.Unregister(v)
			if err_ != nil {
				// TODO: log it
			}
		}

		if mns != nil {
			err_ := mns.Close()
			if err_ != nil {
				// TODO: log it
			}
		}

		if mps != nil {
			err_ := mps.Close()
			if err_ != nil {
				// TODO: log it
			}
		}

		v, ok := obj.(common.Object)
		if ok {
			err_ := v.Close()
			if err_ != nil {
				// TODO: log it
			}
		}
	}()

	if types == common.InstT.DEFAULT {
		producer, _ = obj.(broker.Producer)
		consumer, _ = obj.(broker.Consumer)
		store, _ = obj.(backend.Store)
		reporter, _ = obj.(backend.Reporter)
	} else {
		if types&common.InstT.PRODUCER == common.InstT.PRODUCER {
			producer, ok = obj.(broker.Producer)
			if !ok {
				err = errors.New("producer is not found")
				return
			}
		}
		if types&common.InstT.CONSUMER == common.InstT.CONSUMER {
			consumer, ok = obj.(broker.Consumer)
			if !ok {
				err = errors.New("consumer is not found")
				return
			}
		}
		if types&common.InstT.STORE == common.InstT.STORE {
			store, ok = obj.(backend.Store)
			if !ok {
				err = errors.New("store is not found")
				return
			}
		}
		if types&common.InstT.REPORTER == common.InstT.REPORTER {
			reporter, ok = obj.(backend.Reporter)
			if !ok {
				err = errors.New("reporter is not found")
				return
			}
		}
	}

	// do not overwrite existing objects
	if me.producer != nil {
		producer = nil
	}
	if me.consumer != nil {
		consumer = nil
	}
	if me.reporter != nil {
		reporter = nil
	}
	if me.store != nil {
		store = nil
	}

	if consumer != nil {
		mps, err = newMappers()
		if err != nil {
			return
		}

		for remain := me.cfg.Mappers_; remain > 0; remain-- {
			// TODO: handle events channel
			receipts := make(chan broker.Receipt, 10)
			tasks, err_ := consumer.AddListener(receipts)
			if err_ != nil && err == nil {
				err = err_
				return
			}

			mps.more(tasks, receipts)
			if err != nil {
				return
			}
		}
	}

	if store != nil {
		mns, err = newMonitors(store)
		if err != nil {
			return
		}

		var remain int
		remain, err = mns.more(me.cfg.Monitors_)
		if err != nil {
			return
		}

		if remain > 0 {
			err = errors.New(fmt.Sprintf("Unable to allocate monitors %v", remain))
			return
		}
	}

	// bridge reporter and mappers
	{
		var _m *_mappers = me.mappers
		if mps != nil {
			_m = mps
		}
		var _r backend.Reporter = me.reporter
		if reporter != nil {
			_r = reporter
		}
		if _r != nil && _m != nil {
			err = _r.Report(_m.reports())
			if err != nil {
				return
			}
		}
	}

	// get an id
	for {
		id = rand.Int()
		if _, ok := me.objs[id]; !ok {
			break
		}
	}

	attach_event := func(in []int, o interface{}) (out []int, err error) {
		out = in
		if reflect.ValueOf(o).IsNil() {
			return
		}

		v, ok := o.(common.Object)
		if !ok {
			return
		}

		events, err := v.Events()
		if err != nil {
			return
		}

		var eid int
		for _, e := range events {
			eid, err = me.eventMux.Register(e, 0)
			if err != nil {
				return
			}
			out = append(out, eid)
		}
		return
	}

	eids, err = attach_event(eids, obj)
	if err != nil {
		return
	}
	eids, err = attach_event(eids, mps)
	if err != nil {
		return
	}
	eids, err = attach_event(eids, mns)
	if err != nil {
		return
	}

	// --- nothing should throw error below ---

	// attach objects to app
	if producer != nil {
		me.producer = producer
		used |= common.InstT.PRODUCER
	}
	if consumer != nil {
		me.mappers = mps
		me.consumer = consumer
		used |= common.InstT.CONSUMER
	}
	if reporter != nil {
		me.reporter = reporter
		used |= common.InstT.REPORTER
	}
	if store != nil {
		me.monitors = mns
		me.store = store
		used |= common.InstT.STORE
	}

	me.objs[id] = &_object{
		used: used,
		obj:  obj,
	}

	return
}

func (me *_app) Call(name string, opt *meta.Option, args ...interface{}) (reports <-chan meta.Report, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	// TODO: attach Option to meta.Task
	if me.producer == nil {
		err = errors.New("producer is not initialized")
		return
	}

	t, err := me.invoker.ComposeTask(name, args...)
	if err != nil {
		return
	}

	// blocking call
	err = me.producer.Send(t)
	if err != nil {
		return
	}

	if opt != nil && opt.IgnoreReport_ || me.monitors == nil {
		return
	}

	reports, err = me.monitors.check(t)
	if err != nil {
		return
	}

	return
}

func (me *_app) Listen(targets, level, expected_id int) (id int, events <-chan *common.Event, err error) {
	// the implementation below
	// refers to 'ReadMostly' example in sync/atomic

	me.eventOutLock.Lock()
	defer me.eventOutLock.Unlock()

	listener := &_eventListener{
		targets: targets,
		level:   level,
		events:  make(chan *common.Event, 10),
	}

	m := me.eventOut.Load().(map[int]*_eventListener)
	// get an identifier
	id = expected_id
	for {
		_, ok := m[id]
		if !ok {
			break
		}
		id = rand.Int()
	}

	// copy a new map
	m_ := make(map[int]*_eventListener)
	for k, _ := range m {
		m_[k] = m[k]
	}
	m_[id] = listener
	me.eventOut.Store(m_)
	events = listener.events
	return
}

func (me *_app) StopListen(id int) (err error) {
	// the implementation below
	// refers to 'ReadMostly' example in sync/atomic

	me.eventOutLock.Lock()
	defer me.eventOutLock.Unlock()

	m := me.eventOut.Load().(map[int]*_eventListener)
	_, ok := m[id]
	if !ok {
		err = errors.New(fmt.Sprintf("ID not found:%v", id))
		return
	}

	// we don't close this channel,
	// or we might send to a closed channel
	m_ := make(map[int]*_eventListener)
	for k, _ := range m {
		m_[k] = m[k]
	}

	delete(m_, id)
	me.eventOut.Store(m_)
	return
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
