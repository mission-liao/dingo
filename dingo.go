package dingo

import (
	// standard
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	// internal
	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
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
	// - name: name of tasks
	// - fn: the function that actually perform the task.
	// - count: count of workers to be initialized.
	// - share: the count of workers sharing one report channel
	// - taskEnc, reportEnc: id of encoding method for 'transport.Task' and 'transport.Report'
	//
	// returns ->
	// - remain: remaining count of workers that not initialized.
	// - err: any error produced
	Register(name string, fn interface{}, count, share int, taskEnc, reportEnc int16) (remain int, err error)

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

	//
	Init(cfg Config) (err error)

	// send a task
	//
	Call(name string, opt *transport.Option, args ...interface{}) (<-chan *transport.Report, error)

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
	invoker transport.Invoker

	cfg          Config
	objsLock     sync.RWMutex
	objs         map[int]*_object
	eventMux     *common.Mux
	eventOut     atomic.Value
	eventOutLock sync.Mutex
	b            Bridge
	mash         *transport.Marshallers

	// internal routines
	mappers *_mappers
	workers *_workers
	events  *common.Routines
}

//
// factory function
//
func NewApp(nameOfBridge string) (app App, err error) {
	v := &_app{
		objs:     make(map[int]*_object),
		invoker:  transport.NewDefaultInvoker(),
		eventMux: common.NewMux(),
		events:   common.NewRoutines(),
		mash:     transport.NewMarshallers(),
	}
	v.b = NewBridge(nameOfBridge, v.mash)

	// refer to 'ReadMostly' example in sync/atomic
	v.eventOut.Store(make(map[int]*_eventListener))

	// should only initiate 1 routine for errors,
	// or the order of reported errors would not be
	// preserved.
	go v._event_routine_(v.events.New(), v.events.Wait(), v.eventMux.Out())

	remain, err := v.eventMux.More(1)
	if err != nil || remain != 0 {
		err = errors.New(fmt.Sprintf("Unable to allocate mux routine: %v", remain))
	}

	// init mappers
	v.mappers, err = newMappers()
	if err != nil {
		return
	}
	err = v.attachEvents(v.mappers)
	if err != nil {
		return
	}

	// init workers
	v.workers, err = newWorkers()
	if err != nil {
		return
	}
	err = v.attachEvents(v.workers)
	if err != nil {
		return
	}

	app = v
	return
}

//
// private
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

func (me *_app) attachEvents(obj common.Object) (err error) {
	if obj == nil {
		return
	}

	eids := []int{}
	defer func() {
		if err == nil {
			return
		}

		for _, id := range eids {
			_, err_ := me.eventMux.Unregister(id)
			if err_ != nil {
				// TODO: log it
			}
		}
	}()

	events, err := obj.Events()
	if err != nil {
		return
	}

	for _, e := range events {
		id, err_ := me.eventMux.Register(e, 0)
		if err_ != nil {
			err = err_
			break
		}
		eids = append(eids, id)
	}
	return
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

func (me *_app) Register(name string, fn interface{}, count, share int, mshTask, mshReport int16) (remain int, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	var (
		tasks   <-chan *transport.Task
		reports []<-chan *transport.Report
	)

	// set encoder/decoder
	err = me.mash.Register(name, fn, mshTask, mshReport)
	if err != nil {
		return
	}

	err = me.b.Register(name, fn)
	if err != nil {
		return
	}

	if me.b.Exists(common.InstT.NAMED_CONSUMER) {
		receipts := make(chan *broker.Receipt, 10)
		tasks, err = me.b.AddNamedListener(name, receipts)
		if err != nil {
			return
		}
		reports, remain, err = me.workers.allocate(name, fn, tasks, receipts, count, share)
		if err != nil {
			return
		}
	} else if me.b.Exists(common.InstT.CONSUMER) {
		reports, remain, err = me.mappers.allocateWorkers(name, fn, count, share)
		if err != nil {
			return
		}
	}

	for _, v := range reports {
		// id of report channel is ignored
		err = me.b.Report(v)
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
		producer      broker.Producer
		consumer      broker.Consumer
		namedConsumer broker.NamedConsumer
		store         backend.Store
		reporter      backend.Reporter
		ok            bool
	)

	for {
		id = rand.Int()
		if _, ok := me.objs[id]; !ok {
			break
		}
	}

	defer func() {
		me.objs[id] = &_object{
			used: used,
			obj:  obj,
		}
	}()

	if types == common.InstT.DEFAULT {
		producer, _ = obj.(broker.Producer)
		consumer, _ = obj.(broker.Consumer)
		namedConsumer, _ = obj.(broker.NamedConsumer)
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
			namedConsumer, ok = obj.(broker.NamedConsumer)
			if !ok {
				consumer, ok = obj.(broker.Consumer)
				if !ok {
					err = errors.New("consumer is not found")
					return
				}
			}
		}
		if types&common.InstT.NAMED_CONSUMER == common.InstT.NAMED_CONSUMER {
			namedConsumer, ok = obj.(broker.NamedConsumer)
			if !ok {
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

	if producer != nil {
		err = me.b.AttachProducer(producer)
		if err != nil && types != common.InstT.DEFAULT {
			return
		}
		used |= common.InstT.PRODUCER
	}
	if consumer != nil || namedConsumer != nil {
		err = me.b.AttachConsumer(consumer, namedConsumer)
		if err != nil && types != common.InstT.DEFAULT {
			return
		}
		used |= common.InstT.CONSUMER
	}
	if reporter != nil {
		err = me.b.AttachReporter(reporter)
		if err != nil && types != common.InstT.DEFAULT {
			return
		}
		used |= common.InstT.REPORTER
	}
	if store != nil {
		err = me.b.AttachStore(store)
		if err != nil && types != common.InstT.DEFAULT {
			return
		}
		used |= common.InstT.STORE
	}

	return
}

func (me *_app) Init(cfg Config) (err error) {
	var (
		remain int
		tasks  <-chan *transport.Task
	)
	// integrate mappers and broker.Consumer
	if me.b.Exists(common.InstT.CONSUMER) {
		for remain = cfg.Mappers_; remain > 0; remain-- {
			receipts := make(chan *broker.Receipt, 10)
			tasks, err = me.b.AddListener(receipts)
			if err != nil {
				return
			}

			me.mappers.more(tasks, receipts)
		}
	}

	return
}

func (me *_app) Call(name string, opt *transport.Option, args ...interface{}) (reports <-chan *transport.Report, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	// TODO: attach Option to transport.Task
	t, err := me.invoker.ComposeTask(name, args)
	if err != nil {
		return
	}

	// a blocking call
	err = me.b.SendTask(t)
	if err != nil {
		return
	}

	if opt != nil && opt.IgnoreReport_ {
		return
	}

	reports, err = me.b.Poll(t)
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
