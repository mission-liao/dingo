package dingo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	tasks    <-chan *Task
	rs       *Routines
	reports  []chan *Report
}

//
// worker container
//

type _workers struct {
	workersLock sync.Mutex
	workers     atomic.Value
	events      chan *Event
	eventMux    *mux
	trans       *fnMgr
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
// - reports: array of channels of 'Report'
// - err: any error
func (wrk *_workers) allocate(
	name string,
	tasks <-chan *Task,
	receipts chan<- *TaskReceipt,
	count, share int,
) (reports []<-chan *Report, remain int, err error) {
	var (
		w   *worker
		eid int
	)
	defer func() {
		if err == nil {
			remain, reports, err = wrk.more(name, count, share)
		}

		if err != nil {
			if eid != 0 {
				if _, err_ := wrk.eventMux.Unregister(eid); err_ != nil {
					// TODO: log it
				}
			}

			if w != nil {
				w.rs.Close()
			}
		}
	}()

	err = func() (err error) {
		wrk.workersLock.Lock()
		defer wrk.workersLock.Unlock()

		ws := wrk.workers.Load().(map[string]*worker)

		if _, ok := ws[name]; ok {
			err = fmt.Errorf("name %v exists", name)
			return
		}

		// initiate controlling channle
		w = &worker{
			receipts: receipts,
			tasks:    tasks,
			rs:       NewRoutines(),
			reports:  make([]chan *Report, 0, 10),
		}

		if eid, err = wrk.eventMux.Register(w.rs.Events(), 0); err != nil {
			return
		}

		// -- this line below should never throw any error --

		nws := make(map[string]*worker)
		for k := range ws {
			nws[k] = ws[k]
		}
		nws[name] = w
		wrk.workers.Store(nws)
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
func (wrk *_workers) more(name string, count, share int) (remain int, reports []<-chan *Report, err error) {
	remain = count
	if count <= 0 || share < 0 {
		err = fmt.Errorf("invalid count/share is provided %v", count, share)
		return
	}

	reports = make([]<-chan *Report, 0, remain)

	// locking
	ws := wrk.workers.Load().(map[string]*worker)

	// checking existence of Id
	w, ok := ws[name]
	if !ok {
		err = fmt.Errorf("%d group of worker not found")
		return
	}

	add := func() (r chan *Report) {
		r = make(chan *Report, 10)
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
		go wrk.workerRoutine(
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

func (wrk *_workers) Expect(types int) (err error) {
	if types != ObjT.Worker {
		err = fmt.Errorf("Unsupported types: %v", types)
		return
	}

	return
}

func (wrk *_workers) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		wrk.events,
	}, nil
}

func (wrk *_workers) Close() (err error) {
	wrk.workersLock.Lock()
	defer wrk.workersLock.Unlock()

	// stop all workers routine
	ws := wrk.workers.Load().(map[string]*worker)
	for _, v := range ws {
		v.rs.Close()
		for _, r := range v.reports {
			// closing report to raise quit signal
			close(r)
		}
	}
	wrk.workers.Store(make(map[string]*worker))

	return
}

// factory function
func newWorkers(trans *fnMgr, hooks exHooks) (w *_workers, err error) {
	w = &_workers{
		events:   make(chan *Event, 10),
		eventMux: newMux(),
		trans:    trans,
		hooks:    hooks,
	}

	w.workers.Store(make(map[string]*worker))

	remain, err := w.eventMux.More(1)
	if err == nil && remain != 0 {
		err = fmt.Errorf("Unable to allocate mux routine:%v")
	}
	w.eventMux.Handle(func(val interface{}, _ int) {
		w.events <- val.(*Event)
	})

	return
}

//
// worker routine
//

func (wrk *_workers) workerRoutine(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *Event,
	tasks <-chan *Task,
	receipts chan<- *TaskReceipt,
	reports chan<- *Report,
) {
	defer wait.Done()
	rep := func(task *Task, status int16, payload []interface{}, err error, alreadySent bool) (sent bool) {
		sent = alreadySent
		if task.Option().IgnoreReport() {
			return
		}

		var (
			r    *Report
			err_ error
		)
		if r, err_ = task.composeReport(status, payload, err); err_ != nil {
			if r, err_ = task.composeReport(Status.Fail, nil, NewErr(0, err_)); err_ != nil {
				events <- NewEventFromError(ObjT.Worker, err_)
				return
			}
		}

		if r.Done() || r.Option().MonitorProgress() {
			if !sent {
				sent = true
				wrk.hooks.ReporterHook(ReporterEvent.BeforeReport, task)
			}

			reports <- r
		}

		return
	}
	call := func(t *Task) {
		reported := false
		defer func() {
			if r := recover(); r != nil {
				reported = rep(t, Status.Fail, nil, NewErr(ErrCode.Panic, fmt.Errorf("%v", r)), reported)
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
		if ret, err = wrk.trans.Call(t); err != nil {
			// compose a report -- done / fail
			status = Status.Fail
			events <- NewEventFromError(ObjT.Worker, err_)
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
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				break clean
			}
			call(t)
		default:
			break clean
		}
	}
}
