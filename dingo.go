package dingo

import (
	"sync"
	"time"

	"./backend"
	"./broker"
	"./task"
)

type Result interface {
}

type App interface {
	Init() error
	Uninit() error

	Call(name string, args ...interface{}) (chan task.Report, error)
}

type _monitee struct {
	t    task.Task
	r    chan task.Report
	last task.Report
}

type app struct {
	backend backend.Backend
	broker  broker.Broker
	invoker task.Invoker

	// monitoring routine
	quit, done chan int
	mLock      sync.Mutex
	mTask      map[string]*_monitee
}

func NewApp() App {
	return &app{}
}

//
// App interface
//

func (me *app) Init() error {
	quit = make(chan int) // quit signal, blocking
	done = make(chan int) // done signal, blocking

	// create a monitor routine
	go func(q, d) {
		// create a ticker
		// TODO: configurable duration
		ticker := time.NewTicker(5 * time.Second)

		for {
			select {
			case ticker.C:
				if me.backend == nil {
					break
				}

				me.mLock.Lock()
				defer me.mLock.Unlock()

				for k, t := range me.tasks {
					// get status from backend
					r, err := me.backend.Poll(t.t, t.last)
					if !r.Identical(t.last) {
						t.last = r
						t.r <- r
					}
				}
			case q:
				ticker.Stop()
				// TODO: purge tasks

				{
					me.mLock.Lock()
					defer me.mLock.Unlock()

					// release channels
					for k, v := range me.mTask {
						close(v.r)
					}
				}
				d <- 1
			}
		}
	}(quit, done)
}

func (me *app) Uninit() error {
	// send quit signal
	quit <- 1

	// wait for done signal
	<-done
}

func (me *app) Call(name string, ignoreReport bool, args ...interface{}) (r chan task.Report, err error) {
	// lazy init of invoker
	if me.invoker == nil {
		me.invoker = task.NewDefaultInvoker()
	}

	t, err := me.invoker.ComposeTask(name, args)
	if err != nil {
		return
	}

	err = me.broker.Send(t)
	if err != nil {
		return
	}

	if ignoreReport {
		return
	}

	// register task.Task to a monitor go routine
	r = make(chan task.Report, 1)
	{
		me.mLock.Lock()
		defer me.mLock.Unlock()

		me.mTask[t.GetId()] = &_monitee{t, r}
	}

	return
}
