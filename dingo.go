package dingo

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

/*
 Types of instances
*/
var InstT = struct {
	DEFAULT        int
	REPORTER       int
	STORE          int
	PRODUCER       int
	CONSUMER       int
	MAPPER         int
	WORKER         int
	BRIDGE         int
	NAMED_CONSUMER int
	ALL            int
}{
	0,
	(1 << 0),
	(1 << 1),
	(1 << 2),
	(1 << 3),
	(1 << 4),
	(1 << 5),
	(1 << 6),
	(1 << 7),
	int(^uint(0) >> 1), // Max int
}

type _eventListener struct {
	targets, level int
	events         chan *Event
}

type _object struct {
	used int
	obj  interface{}
}

/*
 */
type App struct {
	cfg          Config
	objsLock     sync.RWMutex
	objs         map[int]*_object
	eventMux     *mux
	eventOut     atomic.Value
	eventOutLock sync.Mutex
	b            bridge
	trans        *mgr
	mappers      *_mappers
	workers      *_workers
}

/*
 "nameOfBridge" refers to different modes of dingo:
  - "local": an App works in local mode, which is similar to other background worker framework.
  - "remote": an App works in remote mode, brokers(ex. AMQP...) and backends(ex. redis..., if required) would be needed to work.
*/
func NewApp(nameOfBridge string, cfg *Config) (app *App, err error) {
	v := &App{
		objs:     make(map[int]*_object),
		eventMux: newMux(),
		trans:    newMgr(),
		cfg:      *cfg,
	}
	v.b = newBridge(nameOfBridge, v.trans)

	// refer to 'ReadMostly' example in sync/atomic
	v.eventOut.Store(make(map[int]*_eventListener))
	v.eventMux.Handle(func(val interface{}, _ int) {
		e := val.(*Event)
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
	v.mappers, err = newMappers(v.trans, v.b.(exHooks))
	if err != nil {
		return
	}
	err = v.attachEvents(v.mappers)
	if err != nil {
		return
	}

	// init workers
	v.workers, err = newWorkers(v.trans, v.b.(exHooks))
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

func (me *App) attachEvents(obj Object) (err error) {
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

	// stop consuming more request first
	chk(me.b.StopAllListeners())

	// further reporter(s) should be reported to Reporter(s)
	// after workers/mappers shutdown

	// shutdown mappers
	chk(me.mappers.Close())

	// shutdown workers
	chk(me.workers.Close())

	// stop reporter/store
	for _, v := range me.objs {
		s, ok := v.obj.(Object)
		if ok {
			chk(s.Close())
		}
	}

	// shutdown mux
	me.eventMux.Close()

	// shutdown the monitor of error channels
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
 Marshaller and Invoker.

 You can pick any builtin Invoker(s)/Marshaller(s) combined with your customized one:

   app.AddMarshaller(3, &struct{JsonSafeMarshaller, __your_customized_invoker__})

 "expectedId" is the expected identifier of this Marshaller, which could be useful when you
 need to sync the Marshaller-ID between producers and consumers. 0~3 are occupied by builtin
 Marshaller(s). Suggested "expectedId" should begin from 100.
*/
func (me *App) AddMarshaller(expectedId int, m Marshaller) error {
	return me.trans.AddMarshaller(expectedId, m)
}

/*
 Register a customized IDMaker, input should be an object implements IDMaker.

 You can register different id-makers to different tasks, internally, dingo would take both
 (name, id) as identity of a task.

 The requirement of IDMaker:
  - uniqueness of generated string among all generated tasks.
  - routine(thread) safe.

 The default IDMaker used by dingo is implemented by uuid4.
*/
func (me *App) AddIdMaker(expectedId int, m IDMaker) error {
	return me.trans.AddIdMaker(expectedId, m)
}

/*
 register a worker function

 parameters:
  - name: name of tasks
  - fn: the function that actually perform the task.
  - taskMash, reportMash: id of Marshaller for 'Task' and 'Report'
  - idmaker: id of IDMaker you would like to use when generating tasks.

 returns:
  - err: any error produced
*/
func (me *App) Register(name string, fn interface{}, taskMash, reportMash, idmaker int) (err error) {
	err = me.trans.Register(name, fn, taskMash, reportMash, idmaker)
	if err != nil {
		return
	}

	err = me.b.ProducerHook(ProducerEvent.DeclareTask, name)
	if err != nil {
		return
	}

	return
}

/*
 allocate more workers. When your Consumer(s) implement NamedConsumer, a new listener (to brokers)
 would be allocated each time you call this function. All allocated workers would serve
 that listener.

 If you want to open more channels to consume from brokers, just call this function multiple
 times.

 parameters:
  - name: the name of tasks.
  - count: count of workers to be initialized.
  - share: the count of workers sharing one report channel.

 returns:
  - remain: remaining count of workers that failed to initialize.
  - err: any error produced
*/
func (me *App) Allocate(name string, count, share int) (remain int, err error) {
	// check if this name register
	_, err = me.trans.GetOption(name)
	if err != nil {
		return
	}

	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	var (
		tasks   <-chan *Task
		reports []<-chan *Report
	)

	if me.b.Exists(InstT.NAMED_CONSUMER) {
		receipts := make(chan *TaskReceipt, 10)
		tasks, err = me.b.AddNamedListener(name, receipts)
		if err != nil {
			return
		}
		reports, remain, err = me.workers.allocate(name, tasks, receipts, count, share)
		if err != nil {
			return
		}
	} else if me.b.Exists(InstT.CONSUMER) {
		reports, remain, err = me.mappers.allocateWorkers(name, count, share)
		if err != nil {
			return
		}
	} else {
		err = errors.New("there is no consumer attached")
		return
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
func (me *App) SetOption(name string, opt *Option) error {
	return me.trans.SetOption(name, opt)
}

/*
 Attach an instance, instance could be any instance implementing
 Reporter, Backend, Producer, Consumer.

 parameters:
  - obj: object to be attached
  - types: interfaces contained in 'obj', refer to dingo.InstT
 returns:
  - id: identifier assigned to this object, 0 is invalid value
  - err: errors

 For a producer, the right combination of "types" is
 InstT.PRODUCER|InstT.STORE, if reporting is not required,
 then only InstT.PRODUCER is used.

 For a consumer, the right combination of "types" is
 InstT.CONSUMER|InstT.REPORTER, if reporting is not reuqired(make sure there is no producer await),
 then only InstT.CONSUMER is used.
*/
func (me *App) Use(obj interface{}, types int) (id int, used int, err error) {
	me.objsLock.Lock()
	defer me.objsLock.Unlock()

	var (
		producer      Producer
		consumer      Consumer
		namedConsumer NamedConsumer
		store         Store
		reporter      Reporter
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

	if types == InstT.DEFAULT {
		producer, _ = obj.(Producer)
		consumer, _ = obj.(Consumer)
		namedConsumer, _ = obj.(NamedConsumer)
		store, _ = obj.(Store)
		reporter, _ = obj.(Reporter)
	} else {
		if types&InstT.PRODUCER == InstT.PRODUCER {
			producer, ok = obj.(Producer)
			if !ok {
				err = errors.New("producer is not found")
				return
			}
		}
		if types&InstT.CONSUMER == InstT.CONSUMER {
			namedConsumer, ok = obj.(NamedConsumer)
			if !ok {
				consumer, ok = obj.(Consumer)
				if !ok {
					err = errors.New("consumer is not found")
					return
				}
			}
		}
		if types&InstT.NAMED_CONSUMER == InstT.NAMED_CONSUMER {
			namedConsumer, ok = obj.(NamedConsumer)
			if !ok {
			}
		}
		if types&InstT.STORE == InstT.STORE {
			store, ok = obj.(Store)
			if !ok {
				err = errors.New("store is not found")
				return
			}
		}
		if types&InstT.REPORTER == InstT.REPORTER {
			reporter, ok = obj.(Reporter)
			if !ok {
				err = errors.New("reporter is not found")
				return
			}
		}
	}

	if producer != nil {
		err = me.b.AttachProducer(producer)
		if err != nil && types != InstT.DEFAULT {
			return
		}
		used |= InstT.PRODUCER
	}
	if consumer != nil || namedConsumer != nil {
		err = me.b.AttachConsumer(consumer, namedConsumer)
		if err != nil && types != InstT.DEFAULT {
			return
		}
		if err == nil && me.b.Exists(InstT.CONSUMER) {
			var (
				remain int
				tasks  <-chan *Task
			)
			for remain = me.cfg.Mappers_; remain > 0; remain-- {
				receipts := make(chan *TaskReceipt, 10)
				tasks, err = me.b.AddListener(receipts)
				if err != nil {
					return
				}

				me.mappers.more(tasks, receipts)
			}
		}
		used |= InstT.CONSUMER
	}
	if reporter != nil {
		err = me.b.AttachReporter(reporter)
		if err != nil && types != InstT.DEFAULT {
			return
		}
		used |= InstT.REPORTER
	}
	if store != nil {
		err = me.b.AttachStore(store)
		if err != nil && types != InstT.DEFAULT {
			return
		}
		used |= InstT.STORE
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
func (me *App) Call(name string, opt *Option, args ...interface{}) (reports <-chan *Report, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	if opt == nil {
		opt, err = me.trans.GetOption(name)
		if err != nil {
			return
		}
	}

	t, err := me.trans.ComposeTask(name, opt, args)
	if err != nil {
		return
	}

	// polling before calling.
	//
	// if we poll after calling, we may lose some report if
	// the task finished very quickly.
	if !opt.IgnoreReport() {
		reports, err = me.b.Poll(t)
		if err != nil {
			return
		}
	}

	// a blocking call to broker component
	err = me.b.SendTask(t)
	if err != nil {
		return
	}

	return
}

/*
 Get the channel to receive events from 'dingo'.

 "targets" are instances you want to monitor, they include:
  - InstT.REPORTER: the Reporter instance attached to this App.
  - InstT.STORE: the Store instance attached to this App.
  - InstT.PRODUCER: the Producer instance attached to this App.
  - InstT.CONSUMER: the Consumer/NamedConsumer instance attached to this App.
  - InstT.MAPPER: the internal component, turn if on when debug.
  - InstT.WORKER: the internal component, turn it on when debug.
  - InstT.BRIDGE: the internal component, turn it on when debug.
  - InstT.ALL: every instance.
 They are bit flags and can be combined as "targets", like:
  InstT.BRIDGE | InstT.WORKER | ...

 "level" are minimal severity level expected, include:
  - ErrLvl.DEBUG
  - ErrLvl.INFO
  - ErrLvl.WARNING
  - ErrLvl.Error

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
func (me *App) Listen(targets, level, expectedId int) (id int, events <-chan *Event, err error) {
	// the implementation below
	// refers to 'ReadMostly' example in sync/atomic

	me.eventOutLock.Lock()
	defer me.eventOutLock.Unlock()

	listener := &_eventListener{
		targets: targets,
		level:   level,
		events:  make(chan *Event, 10),
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
