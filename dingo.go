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

type _object struct {
	used int
	obj  interface{}
}

type App struct {
	cfg          Config
	objsLock     sync.RWMutex
	objs         map[int]*_object
	eventMux     *common.Mux
	eventOut     atomic.Value
	eventOutLock sync.Mutex
	b            bridge
	trans        *transport.Mgr
	mappers      *_mappers
	workers      *_workers
}

/*
 "nameOfBridge" refers to different modes of dingo:
  - "local": an App works in local mode, which is similar to other background worker framework.
  - "remote": an App works in remote mode, brokers(ex. AMQP...) and backends(ex. redis..., if required) would be needed to work.
*/
func NewApp(nameOfBridge string) (app *App, err error) {
	v := &App{
		objs:     make(map[int]*_object),
		eventMux: common.NewMux(),
		trans:    transport.NewMgr(),
	}
	v.b = newBridge(nameOfBridge, v.trans)

	// refer to 'ReadMostly' example in sync/atomic
	v.eventOut.Store(make(map[int]*_eventListener))
	v.eventMux.Handle(func(val interface{}, _ int) {
		e := val.(*common.Event)
		m := v.eventOut.Load().(map[int]*_eventListener)
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
	})

	remain, err := v.eventMux.More(1)
	if err != nil || remain != 0 {
		err = errors.New(fmt.Sprintf("Unable to allocate mux routine: %v", remain))
	}

	// init mappers
	v.mappers, err = newMappers(v.trans)
	if err != nil {
		return
	}
	err = v.attachEvents(v.mappers)
	if err != nil {
		return
	}

	// init workers
	v.workers, err = newWorkers(v.trans)
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

func (me *App) attachEvents(obj common.Object) (err error) {
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

/*
 Release this instance. All reporting channels are closed after returning. However, those sent tasks/reports wouldn't be reclaimed.
*/
func (me *App) Close() (err error) {
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

/*
 Register a customized Marshaller, input should be an object implements both
 transport.Marshaller and transport.Invoker.

 You can pick any builtin Invoker(s)/Marshaller(s) combined with your customized one:

   app.AddMarshaller(3, &struct{transport.JsonSafeMarshaller, __your_customized_invoker__})

 "expectedId" is the expected identifier of this Marshaller, which could be useful when you
 need to sync the Marshaller-ID between producers and consumers. 0~3 are occupied by builtin
 Marshaller(s). Suggested "expectedId" should begin from 100.
*/
func (me *App) AddMarshaller(expectedId int16, m transport.Marshaller) error {
	return me.trans.AddMarshaller(expectedId, m)
}

/*
 register a worker function

 parameters:
  - name: name of tasks
  - fn: the function that actually perform the task.
  - count: count of workers to be initialized.
  - share: the count of workers sharing one report channel.
  - taskMash, reportMash: id of transport.Marshaller for 'transport.Task' and 'transport.Report'

 returns:
  - remain: remaining count of workers that failed to initialize.
  - err: any error produced
*/
func (me *App) Register(name string, fn interface{}, count, share int, taskMash, reportMash int16) (remain int, err error) {
	// TODO: move share, count to another function
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	var (
		tasks   <-chan *transport.Task
		reports []<-chan *transport.Report
	)

	// set encoder/decoder
	err = me.trans.Register(name, fn, taskMash, reportMash)
	if err != nil {
		return
	}

	if me.b.Exists(common.InstT.NAMED_CONSUMER) {
		receipts := make(chan *broker.Receipt, 10)
		tasks, err = me.b.AddNamedListener(name, receipts)
		if err != nil {
			return
		}
		reports, remain, err = me.workers.allocate(name, tasks, receipts, count, share)
		if err != nil {
			return
		}
	} else if me.b.Exists(common.InstT.CONSUMER) {
		reports, remain, err = me.mappers.allocateWorkers(name, count, share)
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

/*
 Set default option used for a worker function.
*/
func (me *App) SetOption(name string, opt *transport.Option) error {
	return me.trans.SetOption(name, opt)
}

/*
 Attach an instance, instance could be any instance implementing
 backend.Reporter, backend.Backend, broker.Producer, broker.Consumer.

 parameters:
  - obj: object to be attached
  - types: interfaces contained in 'obj', refer to dingo.InstT
 returns:
  - id: identifier assigned to this object, 0 is invalid value
  - err: errors

 For a producer, the right combination of "types" is
 common.InstT.PRODUCER|common.InstT.STORE, if reporting is not required,
 then only common.InstT.PRODUCER is used.

 For a consumer, the right combination of "types" is
 common.InstT.CONSUMER|common.InstT.REPORTER, if reporting is not reuqired(make sure there is no producer await),
 then only common.InstT.CONSUMER is used.
*/
func (me *App) Use(obj interface{}, types int) (id int, used int, err error) {
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

// TODO: moving config to NewApp
func (me *App) Init(cfg Config) (err error) {
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

/*
 Initiate a task by providing "name" and execution-"option" of tasks.

 A reporting channel would be returned for callers to monitor the status of tasks,
 and access its result. A suggested procedure to monitor reporting channels is
  finished:
    for {
      select {
        case r, ok := <-report:
        if !ok {
          // dingo.App is closed somewhere else
          break finished
        }

        if r.OK() {
          // the result is ready
          returns := r.Returns()
        }
        if r.Fail() {
          // get error
          err := r.Error()
        }

        if r.Done() {
          break finished
        }
      }
    }

 Multiple reports would be sent for each task:
  - Sent: the task is already sent to brokers.
  - Progress: the consumer received this task, and about to execute it
  - Done: this task is finished without error.
  - Fail: this task failed for some reason.
 Noted: the 'Fail' here doesn't mean your worker function is failed,
 it means "dingo" doesn't execute your worker function properly.
*/
func (me *App) Call(name string, opt *transport.Option, args ...interface{}) (reports <-chan *transport.Report, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	if opt == nil {
		opt, err = me.trans.GetOption(name)
		if err != nil {
			return
		}
	}
	t, err := transport.ComposeTask(name, opt, args)
	if err != nil {
		return
	}

	// a blocking call to broker component
	err = me.b.SendTask(t)
	if err != nil {
		return
	}

	if opt.IgnoreReport() {
		return
	}

	reports, err = me.b.Poll(t)
	if err != nil {
		return
	}

	return
}

/*
 Get the channel to receive events from 'dingo'.

 "targets" are instances you want to monitor, they include:
  - common.InstT.REPORTER: the backend.Reporter instance attached to this App.
  - common.InstT.STORE: the backend.Store instance attached to this App.
  - common.InstT.PRODUCER: the broker.Producer instance attached to this App.
  - common.InstT.CONSUMER: the broker.Consumer/broker.NamedConsumer instance attached to this App.
  - common.InstT.MAPPER: the internal component, turn if on when debug.
  - common.InstT.WORKER: the internal component, turn it on when debug.
  - common.InstT.BRIDGE: the internal component, turn it on when debug.
  - common.InstT.ALL: every instance.
 They are bit flags and can be combined as "targets", like:
  common.InstT.BRIDGE | common.InstT.WORKER | ...

 "level" are minimal severity level expected, include:
  - common.ErrLvl.DEBUG
  - common.ErrLvl.INFO
  - common.ErrLvl.WARNING
  - common.ErrLvl.Error

 "id" is the identity of this event channel, which could be used to stop
 monitoring by calling App.StopListen.

 In general, a dedicated go routine would be initiated for this channel,
 with an infinite for loop, like this:
   for {
     select {
       case e, ok := <-events:
         if !ok {
           // after App.Close(), all reporting channels would be closed,
           // except those channels abandoned by App.StopListen.
           return
         }
         fmt.Printf("%v\n", e)
       case <-quit:
         return
     }
   }
*/
func (me *App) Listen(targets, level, expectedId int) (id int, events <-chan *common.Event, err error) {
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
	id = expectedId
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

/*
 Stop listening events.

 Note: quit signals won't be sent for those channels stopped by App.StopListen.
*/
func (me *App) StopListen(id int) (err error) {
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
