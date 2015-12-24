/*
Package dingo is a task/job <=> worker framework for #golang.

Goal

 This library tries to make tasks invoking / monitoring as easy as possible.
  - any function can be a worker function, as long as types of its parameters are supported.
  - return values of worker functions are also accessible.
  - could be used locally as a queue for background jobs, or remotely as a distributed task queue when connected with AMQP or Redis.

Design

 The design is inspired by
  https://github.com/RichardKnop/machinery
  http://www.celeryproject.org/

 A short version of "how a task is invoked" in this library is:
  -------- caller ---------
  - users input arguments are treated as []interface{}
  - marshall []interface{} to []byte
  - send []byte to broker
  - polling return values from the store
  -------- worker ---------
  - consume []byte from broker
  - unmarshall []byte to []interface{}(underlying types might be different)
  - try to apply type-correction on []interface{}
  - invoking the worker function
  - convert its return values to []interface{}
  - marshall []interface{} to []byte
  - send []byte to the store
  -------- worker ---------
  - the byte stream of return values is ready after polling
  - unmarshall []byte to []interface{}
  - try to apply type-correction on []interface{}
  - return []interface{} to users.

 This library highly relies on reflection to provide flexibility, therefore,
 it may run more slowly than other libraries without using reflection. To overcome this,
 users can provide customized marshaller(s) and invoker(s) without using reflection. These
 customization are task specific, thus users may choose the default marsahller/invoker for
 most tasks, and provide customized marshaller/invoker to those tasks that are performance-critical.

Customization

 These concept are virtualized for extensibility and customization, please refer to
 corresponding reference for details:
  - Generation of ID for new tasks: dingo.IDMaker
  - Parameter Marshalling: dingo.Marshaller
  - Worker Function Invoking: dingo.Invoker
  - Task Publishing/Consuming: dingo.Producer/dingo.Consumer/dingo.NamedConsumer
  - Report Publishing/Consuming: dingo.Reporter/dingo.Store

Parameter Types

 Many parmeter types are supported by this library, except:
  - interface: no way to know the underlying type of an interface.
  - chan: not supported yet.
  - private field in struct, they would be ignored by most encoders. To support this,
    you need to provide a customized marshaller and invoker that can recognize those
	private fields.

TroubleShooting

 It's relative hard to debug a multi-routine library. To know what's wrong inside, users
 can subscribe to receive failure events.(App.Listen)
*/
package dingo

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type _eventListener struct {
	targets, level int
	events         chan *Event
}

type _object struct {
	used int
	obj  interface{}
}

/*App is the core component of dingo.
 */
type App struct {
	cfg          Config
	objsLock     sync.RWMutex
	objs         map[int]*_object
	eventMux     *mux
	eventOut     atomic.Value
	eventOutLock sync.Mutex
	b            bridge
	trans        *fnMgr
	mappers      *_mappers
	workers      *_workers
}

/*NewApp whose "nameOfBridge" refers to different modes of dingo:
  - "local": an App works in local mode, which is similar to other background worker framework.
  - "remote": an App works in remote(distributed) mode, brokers(ex. AMQP...) and backends(ex. redis..., if required) would be needed to work.
*/
func NewApp(nameOfBridge string, cfg *Config) (app *App, err error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	v := &App{
		objs:     make(map[int]*_object),
		eventMux: newMux(),
		trans:    newFnMgr(),
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
		err = fmt.Errorf("Unable to allocate mux routine: %v", remain)
	}

	// init mappers
	v.mappers, err = newMappers(v.trans, v.b.(exHooks))
	if err != nil {
		return
	}
	err = v.attachObject(v.mappers, ObjT.Mapper)
	if err != nil {
		return
	}
	// 'local' mode
	err = v.allocateMappers()
	if err != nil {
		return
	}

	// init workers
	v.workers, err = newWorkers(v.trans, v.b.(exHooks))
	if err != nil {
		return
	}
	err = v.attachObject(v.workers, ObjT.Worker)
	if err != nil {
		return
	}

	app = v
	return
}

func (dg *App) attachObject(obj Object, types int) (err error) {
	if obj == nil {
		return
	}

	err = obj.Expect(types)
	if err != nil {
		return
	}

	eids := []int{}
	defer func() {
		if err == nil {
			return
		}

		for _, id := range eids {
			_, err_ := dg.eventMux.Unregister(id)
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
		id, err_ := dg.eventMux.Register(e, 0)
		if err_ != nil {
			err = err_
			break
		}
		eids = append(eids, id)
	}
	return
}

func (dg *App) allocateMappers() (err error) {
	if dg.b.Exists(ObjT.Consumer) {
		var (
			remain int
			tasks  <-chan *Task
		)
		for remain = dg.cfg.Mappers_; remain > 0; remain-- {
			receipts := make(chan *TaskReceipt, 10)
			tasks, err = dg.b.AddListener(receipts)
			if err != nil {
				return
			}

			dg.mappers.more(tasks, receipts)
		}
	}

	return
}

/*Close is used to release this instance. All reporting channels are closed after returning.
However, those sent tasks/reports wouldn't be reclaimed.
*/
func (dg *App) Close() (err error) {
	dg.objsLock.Lock()
	defer dg.objsLock.Unlock()

	chk := func(err_ error) {
		if err == nil {
			err = err_
		}
	}

	// stop consuming more request first
	chk(dg.b.StopAllListeners())

	// further reporter(s) should be reported to Reporter(s)
	// after workers/mappers shutdown

	// shutdown mappers
	chk(dg.mappers.Close())

	// shutdown workers
	chk(dg.workers.Close())

	// stop reporter/store
	for _, v := range dg.objs {
		s, ok := v.obj.(Object)
		if ok {
			chk(s.Close())
		}
	}

	// shutdown mux
	dg.eventMux.Close()

	// shutdown the monitor of error channels
	func() {
		dg.eventOutLock.Lock()
		defer dg.eventOutLock.Unlock()

		m := dg.eventOut.Load().(map[int]*_eventListener)
		for _, v := range m {
			// send channel-close event
			close(v.events)
		}
		dg.eventOut.Store(make(map[int]*_eventListener))
	}()

	return
}

/*AddMarshaller registers a customized Marshaller, input should be an object implements both
Marshaller and Invoker.

You can pick any builtin Invoker(s)/Marshaller(s) combined with your customized one:

  app.AddMarshaller(3, &struct{JsonSafeMarshaller, __your_customized_invoker__})

"expectedID" is the expected identifier of this Marshaller, which could be useful when you
need to sync the Marshaller-ID between producers and consumers. 0~3 are occupied by builtin
Marshaller(s). Suggested "expectedID" should begin from 100.
*/
func (dg *App) AddMarshaller(expectedID int, m Marshaller) error {
	return dg.trans.AddMarshaller(expectedID, m)
}

/*AddIDMaker registers a customized IDMaker, input should be an object implements IDMaker.

You can register different id-makers to different tasks, internally, dingo would take both
(name, id) as identity of a task.

The requirement of IDMaker:
 - uniqueness of generated string among all generated tasks.
 - routine(thread) safe.

The default IDMaker used by dingo is implemented by uuid4.
*/
func (dg *App) AddIDMaker(expectedID int, m IDMaker) error {
	return dg.trans.AddIDMaker(expectedID, m)
}

/*Register would register a worker function

parameters:
 - name: name of tasks
 - fn: the function that actually perform the task.

returns:
 - err: any error produced
*/
func (dg *App) Register(name string, fn interface{}) (err error) {
	err = dg.trans.Register(name, fn)
	if err != nil {
		return
	}

	err = dg.b.ProducerHook(ProducerEvent.DeclareTask, name)
	if err != nil {
		return
	}

	return
}

/*Allocate would allocate more workers. When your Consumer(s) implement NamedConsumer, a new listener (to brokers)
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
func (dg *App) Allocate(name string, count, share int) (remain int, err error) {
	// check if this name register
	_, err = dg.trans.GetOption(name)
	if err != nil {
		return
	}

	dg.objsLock.RLock()
	defer dg.objsLock.RUnlock()

	var (
		tasks   <-chan *Task
		reports []<-chan *Report
	)

	if dg.b.Exists(ObjT.NamedConsumer) {
		receipts := make(chan *TaskReceipt, 10)
		tasks, err = dg.b.AddNamedListener(name, receipts)
		if err != nil {
			return
		}
		reports, remain, err = dg.workers.allocate(name, tasks, receipts, count, share)
		if err != nil {
			return
		}
	} else if dg.b.Exists(ObjT.Consumer) {
		reports, remain, err = dg.mappers.allocateWorkers(name, count, share)
		if err != nil {
			return
		}
	} else {
		err = errors.New("there is no consumer attached")
		return
	}

	for _, v := range reports {
		// id of report channel is ignored
		err = dg.b.Report(v)
		if err != nil {
			return
		}
	}

	return
}

/*SetOption would set default option used for a worker function.
 */
func (dg *App) SetOption(name string, opt *Option) error {
	return dg.trans.SetOption(name, opt)
}

/*SetMarshaller would set marshallers used for marshalling tasks and reports

parameters:
 - name: name of tasks
 - taskMash, reportMash: id of Marshaller for 'Task' and 'Report'
*/
func (dg *App) SetMarshaller(name string, taskMash, reportMash int) error {
	return dg.trans.SetMarshaller(name, taskMash, reportMash)
}

/*SetIDMaker would set IDMaker used for a specific kind of tasks

parameters:
 - name: name of tasks
 - idmaker: id of IDMaker you would like to use when generating tasks.
*/
func (dg *App) SetIDMaker(name string, id int) error {
	return dg.trans.SetIDMaker(name, id)
}

/*Use is used to attach an instance, instance could be any instance implementing
Reporter, Backend, Producer, Consumer.

parameters:
 - obj: object to be attached
 - types: interfaces contained in 'obj', refer to dingo.ObjT
returns:
 - id: identifier assigned to this object, 0 is invalid value
 - err: errors

For a producer, the right combination of "types" is
ObjT.Producer|ObjT.Store, if reporting is not required,
then only ObjT.Producer is used.

For a consumer, the right combination of "types" is
ObjT.Consumer|ObjT.Reporter, if reporting is not reuqired(make sure there is no producer await),
then only ObjT.Consumer is used.
*/
func (dg *App) Use(obj Object, types int) (id int, used int, err error) {
	dg.objsLock.Lock()
	defer dg.objsLock.Unlock()

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
		if _, ok := dg.objs[id]; !ok {
			break
		}
	}

	defer func() {
		err_ := dg.attachObject(obj.(Object), used)
		if err == nil {
			err = err_
		}

		dg.objs[id] = &_object{
			used: used,
			obj:  obj,
		}
	}()

	if types == ObjT.Default {
		producer, _ = obj.(Producer)
		consumer, _ = obj.(Consumer)
		namedConsumer, _ = obj.(NamedConsumer)
		store, _ = obj.(Store)
		reporter, _ = obj.(Reporter)
	} else {
		if types&ObjT.Producer == ObjT.Producer {
			producer, ok = obj.(Producer)
			if !ok {
				err = errors.New("producer is not found")
				return
			}
		}
		if types&ObjT.Consumer == ObjT.Consumer {
			namedConsumer, ok = obj.(NamedConsumer)
			if !ok {
				consumer, ok = obj.(Consumer)
				if !ok {
					err = errors.New("consumer is not found")
					return
				}
			}
		}
		if types&ObjT.NamedConsumer == ObjT.NamedConsumer {
			namedConsumer, ok = obj.(NamedConsumer)
			if !ok {
			}
		}
		if types&ObjT.Store == ObjT.Store {
			store, ok = obj.(Store)
			if !ok {
				err = errors.New("store is not found")
				return
			}
		}
		if types&ObjT.Reporter == ObjT.Reporter {
			reporter, ok = obj.(Reporter)
			if !ok {
				err = errors.New("reporter is not found")
				return
			}
		}
	}

	if producer != nil {
		err = dg.b.AttachProducer(producer)
		if err != nil && types != ObjT.Default {
			return
		}
		used |= ObjT.Producer
	}
	if consumer != nil || namedConsumer != nil {
		err = dg.b.AttachConsumer(consumer, namedConsumer)
		if err != nil && types != ObjT.Default {
			return
		}

		if err == nil {
			if dg.b.Exists(ObjT.Consumer) {
				used |= ObjT.Consumer
			} else if dg.b.Exists(ObjT.NamedConsumer) {
				used |= ObjT.NamedConsumer
			} else {
				err = errors.New("there is no consumer exists in bridge")
				return
			}
		}

		if err == nil {
			err = dg.allocateMappers()
			if err != nil {
				return
			}
		}
	}
	if reporter != nil {
		err = dg.b.AttachReporter(reporter)
		if err != nil && types != ObjT.Default {
			return
		}
		used |= ObjT.Reporter
	}
	if store != nil {
		err = dg.b.AttachStore(store)
		if err != nil && types != ObjT.Default {
			return
		}
		used |= ObjT.Store
	}

	return
}

/*Call would initiate a task by providing:
 - "name" of tasks
 - execution-"option" of tasks, could be nil
 - argument of corresponding worker function.

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
 - Success: this task is finished without error.
 - Fail: this task failed for some reason.
Noted: the 'Fail' here doesn't mean your worker function is failed,
it means "dingo" doesn't execute your worker function properly.
*/
func (dg *App) Call(name string, opt *Option, args ...interface{}) (reports <-chan *Report, err error) {
	dg.objsLock.RLock()
	defer dg.objsLock.RUnlock()

	if opt == nil {
		opt, err = dg.trans.GetOption(name)
		if err != nil {
			return
		}
	}

	t, err := dg.trans.ComposeTask(name, opt, args)
	if err != nil {
		return
	}

	// polling before calling.
	//
	// if we poll after calling, we may lose some report if
	// the task finished very quickly.
	if !opt.IgnoreReport() {
		reports, err = dg.b.Poll(t)
		if err != nil {
			return
		}
	}

	// a blocking call to broker component
	err = dg.b.SendTask(t)
	if err != nil {
		return
	}

	return
}

/*Listen would subscribe the channel to receive events from 'dingo'.

"targets" are instances you want to monitor, they include:
 - dingo.ObjT.Reporter: the Reporter instance attached to this App.
 - dingo.ObjT.Store: the Store instance attached to this App.
 - dingo.ObjT.Producer: the Producer instance attached to this App.
 - dingo.ObjT.Consumer: the Consumer/NamedConsumer instance attached to this App.
 - dingo.ObjT.Mapper: the internal component, turn if on when debug.
 - dingo.ObjT.Worker: the internal component, turn it on when debug.
 - dingo.ObjT.Bridge: the internal component, turn it on when debug.
 - dingo.ObjT.All: every instance.
They are bit flags and can be combined as "targets", like:
 ObjT.Bridge | ObjT.Worker | ...

"level" are minimal severity level expected, include:
 - dingo.EventLvl.Debug
 - dingo.EventLvl.Info
 - dingo.EventLvl.Warning
 - dingo.EventLvl.Error

"id" is the identity of this event channel, which could be used to stop
monitoring by calling dingo.App.StopListen.

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
func (dg *App) Listen(targets, level, expectedID int) (id int, events <-chan *Event, err error) {
	// the implementation below
	// refers to 'ReadMostly' example in sync/atomic

	dg.eventOutLock.Lock()
	defer dg.eventOutLock.Unlock()

	listener := &_eventListener{
		targets: targets,
		level:   level,
		events:  make(chan *Event, 10),
	}

	m := dg.eventOut.Load().(map[int]*_eventListener)
	// get an identifier
	id = expectedID
	for {
		_, ok := m[id]
		if !ok {
			break
		}
		id = rand.Int()
	}

	// copy a new map
	nm := make(map[int]*_eventListener)
	for k := range m {
		nm[k] = m[k]
	}
	nm[id] = listener
	dg.eventOut.Store(nm)
	events = listener.events
	return
}

/*StopListen would stop listening events.

Note: those channels stopped by App.StopListen wouldn't be closed but only
be reclaimed by GC.
*/
func (dg *App) StopListen(id int) (err error) {
	// the implementation below
	// refers to 'ReadMostly' example in sync/atomic

	dg.eventOutLock.Lock()
	defer dg.eventOutLock.Unlock()

	m := dg.eventOut.Load().(map[int]*_eventListener)
	_, ok := m[id]
	if !ok {
		err = fmt.Errorf("ID not found:%v", id)
		return
	}

	// we don't close this channel,
	// or we might send to a closed channel
	nm := make(map[int]*_eventListener)
	for k := range m {
		nm[k] = m[k]
	}

	delete(nm, id)
	dg.eventOut.Store(nm)
	return
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
