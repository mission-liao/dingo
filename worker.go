package dingo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
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
	receipts chan<- *broker.Receipt
	tasks    <-chan *transport.Task
	rs       *common.Routines
	reports  []chan *transport.Report
}

//
// worker container
//

type _workers struct {
	workersLock sync.Mutex
	workers     atomic.Value
	events      chan *common.Event
	eventMux    *common.Mux
	trans       *transport.Mgr
}

// allocating a new group of workers
//
// parameters:
// - id: identifier of this group of workers share the same function, matcher...
// - match: matcher of this group of workers
// - tasks: input channel
// - receipts: output 'broker.Receipt' channel
// - count: count of workers to be initiated
// - share: the count of workers sharing one report channel
// returns:
// - remain: count of workers remain not initiated
// - reports: array of channels of 'transport.Report'
// - err: any error
func (me *_workers) allocate(
	name string,
	tasks <-chan *transport.Task,
	receipts chan<- *broker.Receipt,
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
			rs:       common.NewRoutines(),
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
// common.Object interface
//

func (me *_workers) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
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
			close(r)
		}
	}
	me.workers.Store(make(map[string]*worker))

	return
}

// factory function
func newWorkers(trans *transport.Mgr) (w *_workers, err error) {
	w = &_workers{
		events:   make(chan *common.Event, 10),
		eventMux: common.NewMux(),
		trans:    trans,
	}

	w.workers.Store(make(map[string]*worker))

	remain, err := w.eventMux.More(1)
	if err == nil && remain != 0 {
		err = errors.New(fmt.Sprintf("Unable to allocate mux routine:%v"))
	}
	w.eventMux.Handle(func(val interface{}, _ int) {
		w.events <- val.(*common.Event)
	})

	return
}

//
// worker routine
//

func (me *_workers) _worker_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *common.Event,
	tasks <-chan *transport.Task,
	receipts chan<- *broker.Receipt,
	reports chan<- *transport.Report,
) {
	defer wait.Done()
	rep := func(task *transport.Task, status int16, payload []interface{}, err error) {
		if task.Option().IgnoreReport() {
			return
		}

		var (
			e    *transport.Error
			r    *transport.Report
			err_ error
		)
		if err != nil {
			e = transport.NewErr(0, err)
		}
		r, err_ = task.ComposeReport(status, payload, e)
		if err_ != nil {
			r, err_ = task.ComposeReport(transport.Status.Fail, nil, transport.NewErr(0, err_))
			if err_ != nil {
				events <- common.NewEventFromError(common.InstT.WORKER, err_)
				return
			}
		}

		reports <- r
	}
	call := func(t *transport.Task) {
		var (
			ret       []interface{}
			err, err_ error
			status    int16
		)

		// compose a report -- sent
		rep(t, transport.Status.Sent, nil, nil)

		// compose a report -- progress
		rep(t, transport.Status.Progress, nil, nil)

		// call the actuall function, where is the magic
		ret, err = me.trans.Call(t)

		// compose a report -- done / fail
		if err != nil {
			status = transport.Status.Fail
			events <- common.NewEventFromError(common.InstT.WORKER, err_)
		} else {
			status = transport.Status.Done
		}
		rep(t, status, ret, err)
	}

	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				goto clean
			}

			if receipts != nil {
				receipts <- &broker.Receipt{
					ID:     t.ID(),
					Status: broker.Status.OK,
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
