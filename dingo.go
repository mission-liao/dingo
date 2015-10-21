package dingo

import (
	// standard
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	// internal
	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/task"
)

var InstT = struct {
	DEFAULT  int
	REPORTER int
	STORE    int
	PRODUCER int
	CONSUMER int
	ALL      int
}{
	0,
	(1 << 0),
	(1 << 1),
	(1 << 2),
	(1 << 3),
	(1 << 4) - 1,
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

	// attach an instance, instance could be any instance of
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
	Call(name string, opt *Option, args ...interface{}) (<-chan task.Report, error)
}

//
// app
//

type _object struct {
	rid  string
	used int
	obj  interface{}
}

type _app struct {
	invoker task.Invoker

	cfg      Config
	objsLock sync.RWMutex
	objs     map[int]*_object
	producer broker.Producer
	consumer broker.Consumer
	store    backend.Store
	reporter backend.Reporter
	receipts chan broker.Receipt

	// internal routines
	mappers  *_mappers
	monitors *_monitors
}

// factory function
//
func NewApp(c Config) (app App, err error) {
	app = &_app{
		receipts: make(chan broker.Receipt, 10),
		objs:     make(map[int]*_object),
		invoker:  task.NewDefaultInvoker(),
		cfg:      c,
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
		if v.used&InstT.CONSUMER == InstT.CONSUMER {
			chk(me.consumer.Stop())
		}

		if v.used&InstT.REPORTER == InstT.REPORTER {
			chk(me.reporter.Unbind(v.rid))
		}

		s, ok := v.obj.(common.Server)
		if ok {
			chk(s.Close())
		}
	}

	// shutdown mappers
	if me.mappers != nil {
		chk(me.mappers.done())
		me.mappers = nil
	}

	// shutdown monitors
	if me.monitors != nil {
		chk(me.monitors.done())
		me.monitors = nil
	}

	close(me.receipts)
	return
}

func (me *_app) Register(m Matcher, fn interface{}, count int) (id string, remain int, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	remain = count
	if me.mappers != nil {
		id, remain, err = me.mappers.allocateWorkers(m, fn, count)
		if err != nil {
			return
		}
	}

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
		ok       bool
	)

	if types == InstT.DEFAULT {
		producer, _ = obj.(broker.Producer)
		consumer, _ = obj.(broker.Consumer)
		store, _ = obj.(backend.Store)
		reporter, _ = obj.(backend.Reporter)
	} else {
		if types&InstT.PRODUCER == InstT.PRODUCER {
			producer, ok = obj.(broker.Producer)
			if !ok {
				err = errors.New("producer is not found")
				return
			}
		}

		if types&InstT.CONSUMER == InstT.CONSUMER {
			consumer, ok = obj.(broker.Consumer)
			if !ok {
				err = errors.New("consumer is not found")
				return
			}
		}

		if types&InstT.STORE == InstT.STORE {
			store, ok = obj.(backend.Store)
			if !ok {
				err = errors.New("store is not found")
				return
			}
		}

		if types&InstT.REPORTER == InstT.REPORTER {
			reporter, ok = obj.(backend.Reporter)
			if !ok {
				err = errors.New("reporter is not found")
				return
			}
		}
	}

	var rid string

	if producer != nil && me.producer == nil {
		me.producer = producer
		used |= InstT.PRODUCER
	}

	if consumer != nil && me.consumer == nil {
		// TODO: handle errs channel
		tasks, _, err_ := consumer.Consume(me.receipts)
		if err_ != nil {
			err = err_
			return
		}

		mp := newMappers(tasks, me.receipts)
		if me.reporter != nil {
			rid, err = me.reporter.Report(mp.reports())
			if err != nil {
				return
			}
		}

		remain, err_ := mp.more(me.cfg.Mappers_)
		if err_ != nil {
			err = err_
			return
		}

		if remain > 0 {
			err = errors.New(fmt.Sprintf("Unable to allocate mappers %v", remain))
			return
		}
		me.mappers = mp
		me.consumer = consumer
		used |= InstT.CONSUMER
	}

	if store != nil && me.monitors == nil {
		mn, err_ := newMonitors(store)
		if err_ != nil {
			err = err_
			return
		}

		remain, err_ := mn.more(me.cfg.Monitors_)
		if err_ != nil {
			err = err_
			return
		}

		if remain > 0 {
			err = errors.New(fmt.Sprintf("Unable to allocate monitors %v", remain))
			return
		}
		me.monitors = mn
		used |= InstT.STORE
	}

	if reporter != nil && me.reporter == nil {
		if me.mappers != nil {
			rid, err = reporter.Report(me.mappers.reports())
			if err != nil {
				return
			}
		}

		me.reporter = reporter
		used |= InstT.REPORTER
	}

	// get an id
	for {
		id = rand.Int()
		if _, ok := me.objs[id]; ok {
			continue
		}

		me.objs[id] = &_object{
			rid:  rid,
			used: used,
			obj:  obj,
		}
		break
	}

	return
}

func (me *_app) Call(name string, opt *Option, args ...interface{}) (reports <-chan task.Report, err error) {
	me.objsLock.RLock()
	defer me.objsLock.RUnlock()

	// TODO: attach Option to task.Task
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
