package dingo

import (
	// standard
	"errors"
	"sync"
	"time"

	// internal
	"./backend"
	"./broker"
	"./task"
)

type App interface {
	Init() error
	Close() error

	// send a task
	Call(name string, args ...interface{}) (chan task.Report, error)

	// hire a set of workers for a pattern
	NewWorkers(match func(string) bool, fn interface{}, count int) (remain int, err error)

	// launch a monitor
	NewMonitor() error

	// launch a mapper -- map tasks to workers
	NewDispatcher() error

	// launch a reducer -- collecting return values and push them to backend
	NewReducer() error
}

type _report struct {
	t *broker.TaskInfo
	r task.Report
}

type _monitee struct {
	t    task.Task
	r    chan task.Report
	last task.Report
}

type _worker struct {
	match func(string) bool
	task  chan broker.TaskInfo
	quits []chan int
}

type app struct {
	backend backend.Backend
	broker  broker.Broker
	invoker task.Invoker

	quit, done chan int

	// monitoring routine
	mTaskLock sync.Mutex
	mTask     map[string]*_monitee

	// dispatch routine
	tasks     chan broker.TaskInfo
	errs      chan broker.ErrInfo
	reports   chan *_report
	mQuit     map[string]chan<- int
	mFinished map[string]<-chan int

	// worker info
	workers []*_worker

	// report channel
}

func NewApp() App {
	return &app{}
}

//
// App interface
//

func (me *app) Init() (err error) {
	// init broker
	me.quit = make(chan int, 1) // quit signal
	me.done = make(chan int, 1) // done signal
	me.tasks = make(chan broker.TaskInfo, 10)
	me.errs = make(chan broker.ErrInfo, 10)
	me.reports = make(chan *_report, 10)

	me.broker = broker.NewBroker()
	err = me.broker.Init()
	if err != nil {
		return
	}

	// init backend
	me.backend = backend.NewBackend()
	err = me.backend.Init()
	if err != nil {
		return
	}

	// init ...
	me.workers = make([]*_worker)

	// create a monitor routine
	go func(q, d) {
		// forever loop
		for {
			select {
			case time.After(5 * time.second):
				// TODO: configurable duration
				if me.backend == nil {
					break
				}

				me.mTaskLock.Lock()
				defer me.mTaskLock.Unlock()

				for k, t := range me.mTask {
					// get status from backend
					r, err := me.backend.Poll(t.t)
					if !r.Valid() {
						// usually, this means the worker
						// doesn't send report yet.

						// TODO: a time out mechanism
						continue
					}
					if !r.Identical(t.last) {
						t.last = r
						t.r <- r
					}
					if r.Done() {
						// magically, it's safe to delete when interating in go
						delete(me.mTask, k)
					}
				}
			case q:
				// TODO: purge tasks

				{
					me.mTaskLock.Lock()
					defer me.mTaskLock.Unlock()

					// release channels
					for k, v := range me.mTask {
						close(v.r)
					}
				}
				d <- 1
			}
		}
	}(me.quit, me.done)

}

func (me *app) Close() (err error) {
	// quit monitor/dispatch routine
	{
		me.quit <- 1
		<-me.done

		me.quit <- 1
		<-me.done
	}

	err = me.broker.Close()
	err_ = me.backend.Close()
	if err == nil {
		err = err_
	}

	close(me.done)
	close(me.quit)
	close(me.tasks)
	close(me.reports)
	close(me.errs)

	return
}

func (me *app) Call(name string, ignoreReport bool, args ...interface{}) (r chan task.Report, err error) {
	if me.broker == nil {
		err = errors.New("Broker is not initialized")
		return
	}

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

	if ignoreReport || me.backend == nil {
		return
	}

	// TODO: when leaving, those un-receiver Poller
	// should be released.
	var (
		r chan<- task.Report
	)

	r, err = me.backend.NewPoller()
	if err != nil {
		// TODO: log it
		return
	}

	// register task.Task to a monitor go routine
	r := make(chan task.Report, 5)
	{
		me.mTaskLock.Lock()
		defer me.mTaskLock.Unlock()

		me.mTask[t.GetId()] = &_monitee{t, r}
	}

	// compose 1st report -- Sent
	{
		var r task.Report

		r, err = t.ComposeReport(task.Status.Sent, nil, nil)
		if err != nil {
			return
		}
		me.reports <- r
	}

	return
}

func (me *_app) NewWorkers(name string, match func(string) bool, fn interface{}, count int) (remain int, err error) {
	// lazy init of invoker
	if me.invoker == nil {
		me.invoker = task.NewDefaultInvoker()
	}

	quits := []chan int{}
	task := make(chan broker.TaskInfo, count)
	for ; count > 0; count-- {
		q := make(chan int, 1)
		//
		go func(quit <-chan int, task <-chan broker.TaskInfo, report chan task.Report, ivk task.Invoker, fn interface{}) {
			for {
				select {
				case t := <-task:
					var (
						r      task.Report
						ret    []interface{}
						err_   error
						status int
					)

					// compose a report -- progress
					r, err_ = t.ComposeReport(task.Status.Progress, nil, nil)
					if err_ != nil {
						r, err_ = t.ComposeReport(task.Status.Fail, err, nil)
						if err_ != nil {
							// TODO: log it
							break
						}
					}
					report <- r

					// call the actuall function, where is the magic
					ret, err_ = ivk.Invoke(fn, t.GetArgs())

					// compose a report -- done / fail
					if err_ != nil {
						status = task.Status.Fail
					} else {
						status = task.Status.Done
					}

					r, err_ = t.ComposeReport(status, err, ret)
					if err_ != nil {
						// TODO: log it
						break
					}
					report <- r

				case <-quit:
					// nothing to clean
					return
				}
			}
		}(q, t, me.reports, me.invoker, fn)

		quits = append(quits, q)
	}

	me.workers = append(me.workers, &_worker{
		match: match,
		task:  task,
		quits: quits,
	})

	return count, nil
}

func (me *_app) NewMapper() error {
	// TODO: keep a count of mapper?
	go func(quit <-chan int, done chan<- int, tasks chan<- broker.TaskInfo) {
		for {
			select {
			case t := <-tasks:
				// find registered worker
				found := false
				for _, v := range me.workers {
					if m.match(t.GetName()) {
						v.task <- t
						found = true
						break
					}
				}

				// compose a receipt
				var rpt broker.Recepit
				if !found {
					rpt = broker.Receipt{
						Status: broker.Status.WORKER_NOT_FOUND,
					}
				} else {
					rpt = broker.Receipt{
						Status: broker.Status.OK,
					}
				}
				t.Done <- rpt

			case <-quit:
				done <- 1
				return
			}
		}
	}(me.quit, me.done)

	return nil
}

func (me *_app) NewReducer() error {
	// TODO: keep a count of reducer
	go func(quit <-chan int, done chan<- int, report <-chan *_report, errs <-chan broker.ErrInfo) {
		for {
			select {
			case r := <-report:
				if me.backend != nil {
					// push result to backend
					err := me.backend.Update(r.r)
					if err != nil {
						// TODO: log it
					}

					// notify receiver that this work has been done
					r.t.Done <- 1
				}
			case err := <-errs:
				// TODO: logit
				break
			case <-quit:
				done <- 1
				return
			}
		}
	}(me.quit, me.done, me.report, me.errs)

	return nil
}
