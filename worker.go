package dingo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mission-liao/dingo/transport"
)

//
// errors
//

var (
	errWorkerNotFound = errors.New("Worker not found")
)

//
// worker
//
// a required record for a group of workers
//

type worker struct {
	receipts chan<- *TaskReceipt
	tasks    <-chan *transport.Task
	rs       *Routines
	reports  []chan *transport.Report
}

//
// worker container
//

type _workers struct {
	workersLock sync.Mutex
	workers     atomic.Value
	events      chan *Event
	eventMux    *Mux
	trans       *transport.Mgr
	hooks       exHooks
}

// allocating a new group of workers
//
// parameters:
// - id: identifier of this group of workers share the same function, matcher...
// - match: matcher of this group of workers
// - tasks: input channel
// - receipts: output 'TaskReceipt' channel
// - count: count of workers to be initiated
// - share: the count of workers sharing one report channel
// returns:
// - remain: count of workers remain not initiated
// - reports: array of channels of 'transport.Report'
// - err: any error
func (me *_workers) allocate(
	name string,
	tasks <-chan *transport.Task,
	receipts chan<- *TaskReceipt,
	count, share int,
) (reports []<-chan *transport.Report, remain int, err error) {
	var (
		w   *worker
		eid int
	)
	defer func() {
		if err == nil {
			remain, reports, err = me.more(name, count, share)
		}

		if err != nil {
			if eid != 0 {
				_, err_ := me.eventMux.Unregister(eid)
				if err_ != nil {
					// TODO: log it
				}
			}

			if w != nil {
				w.rs.Close()
			}
		}
	}()

	err = func() (err error) {
		me.workersLock.Lock()
		defer me.workersLock.Unlock()

		ws := me.workers.Load().(map[string]*worker)

		if _, ok := ws[name]; ok {
			err = errors.New(fmt.Sprintf("name %v exists", name))
			return
		}

		// initiate controlling channle
		w = &worker{
			receipts: receipts,
			tasks:    tasks,
			rs:       NewRoutines(),
			reports:  make([]chan *transport.Report, 0, 10),
		}

		eid, err = me.eventMux.Register(w.rs.Events(), 0)
		if err != nil {
			return
		}

		// -- this line below should never throw any error --

		nws := make(map[string]*worker)
		for k := range ws {
			nws[k] = ws[k]
		}
		nws[name] = w
		me.workers.Store(nws)
		return
	}()

	return
}

// allocating more workers
//
// parameters:
// - name: identifier of this group of workers share the same function, matcher...
// - count: count of workers to be initiated
// - share: count of workers sharing one report channel
// returns:
// - remain: count of workers remain not initiated
// - err: any error
func (me *_workers) more(name string, count, share int) (remain int, reports []<-chan *transport.Report, err error) {
	remain = count
	if count <= 0 || share < 0 {
		err = errors.New(fmt.Sprintf("invalid count/share is provided %v", count, share))
		return
	}

	reports = make([]<-chan *transport.Report, 0, remain)

	// locking
	ws := me.workers.Load().(map[string]*worker)

	// checking existence of Id
	w, ok := ws[name]
	if !ok {
		err = errors.New(fmt.Sprintf("%d group of worker not found"))
		return
	}

	add := func() (r chan *transport.Report) {
		r = make(chan *transport.Report, 10)
		reports = append(reports, r)
		w.reports = append(w.reports, r)
		return
	}

	r := add()
	// initiating workers
	for ; remain > 0; remain-- {
		// re-initialize a report channel
		if share > 0 && remain != count && remain%share == 0 {
			r = add()
		}
		go me._worker_routine_(
			w.rs.New(),
			w.rs.Wait(),
			w.rs.Events(),
			w.tasks,
			w.receipts,
			r,
		)
	}

	return
}

//
// Object interface
//

func (me *_workers) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		me.events,
	}, nil
}

func (me *_workers) Close() (err error) {
	me.workersLock.Lock()
	defer me.workersLock.Unlock()

	// stop all workers routine
	ws := me.workers.Load().(map[string]*worker)
	for _, v := range ws {
		v.rs.Close()
		for _, r := range v.reports {
			// closing report to raise quit signal
			close(r)
		}
	}
	me.workers.Store(make(map[string]*worker))

	return
}

// factory function
func newWorkers(trans *transport.Mgr, hooks exHooks) (w *_workers, err error) {
	w = &_workers{
		events:   make(chan *Event, 10),
		eventMux: NewMux(),
		trans:    trans,
		hooks:    hooks,
	}

	w.workers.Store(make(map[string]*worker))

	remain, err := w.eventMux.More(1)
	if err == nil && remain != 0 {
		err = errors.New(fmt.Sprintf("Unable to allocate mux routine:%v"))
	}
	w.eventMux.Handle(func(val interface{}, _ int) {
		w.events <- val.(*Event)
	})

	return
}

//
// worker routine
//

func (me *_workers) _worker_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *Event,
	tasks <-chan *transport.Task,
	receipts chan<- *TaskReceipt,
	reports chan<- *transport.Report,
) {
	defer wait.Done()
	rep := func(task *transport.Task, status int16, payload []interface{}, err error, alreadySent bool) (sent bool) {
		sent = alreadySent
		if task.Option().IgnoreReport() {
			return
		}

		var (
			r    *transport.Report
			err_ error
		)
		r, err_ = task.ComposeReport(status, payload, err)
		if err_ != nil {
			r, err_ = task.ComposeReport(Status.Fail, nil, transport.NewErr(0, err_))
			if err_ != nil {
				events <- NewEventFromError(InstT.WORKER, err_)
				return
			}
		}

		if r.Done() || r.Option().MonitorProgress() {
			if !sent {
				sent = true
				me.hooks.ReporterHook(ReporterEvent.BeforeReport, task)
			}

			reports <- r
		}

		return
	}
	call := func(t *transport.Task) {
		reported := false
		defer func() {
			if r := recover(); r != nil {
				reported = rep(t, Status.Fail, nil, transport.NewErr(ErrCode.Panic, errors.New(fmt.Sprintf("%v", r))), reported)
			}
		}()

		var (
			ret       []interface{}
			err, err_ error
			status    int16
		)

		// compose a report -- sent
		reported = rep(t, Status.Sent, nil, nil, reported)

		// compose a report -- progress
		reported = rep(t, Status.Progress, nil, nil, reported)

		// call the actuall function, where is the magic
		ret, err = me.trans.Call(t)

		// compose a report -- done / fail
		if err != nil {
			status = Status.Fail
			events <- NewEventFromError(InstT.WORKER, err_)
		} else {
			status = Status.Success
		}
		reported = rep(t, status, ret, err, reported)
	}

	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				goto clean
			}

			if receipts != nil {
				receipts <- &TaskReceipt{
					ID:     t.ID(),
					Status: ReceiptStatus.OK,
				}
			}

			call(t)
		case _, _ = <-quit:
			// nothing to clean
			goto clean
		}
	}
clean:
	finished := false
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				finished = true
				break
			}
			call(t)
		default:
			finished = true
		}
		if finished {
			break
		}
	}
}
