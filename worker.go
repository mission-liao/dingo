package dingo

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/satori/go.uuid"
)

//
// errors
//

var (
	errWorkerNotFound = errors.New("Worker not found")
)

//
// Matcher
//

// 'Matcher' is a role to check if a name belongs to
// this group of workers
type Matcher interface {

	//
	//
	// parameter:
	// - patt: the pattern to be checked.
	// return:
	// - a boolean to represent 'yes' or 'no'.
	Match(patt string) bool
}

type StrMatcher struct {
	name string
}

func (me *StrMatcher) Match(patt string) bool {
	return me.name == patt
}

//
// worker
//
// a required record for a group of workers
//

type worker struct {
	matcher Matcher
	tasks   chan meta.Task
	rs      *common.Routines
	fn      interface{}
}

//
// worker container
//

type _workers struct {
	lock           sync.RWMutex
	workers        map[string]*worker
	reports        chan meta.Report
	events         chan *common.Event
	eventConverter *common.Routines
	eventMux       *common.Mux
}

// allocating a new group of workers
//
// parameters:
// - id: identifier of this group of workers share the same function, matcher...
// - match: matcher of this group of workers
// - fn: the function that worker should called when receiving tasks (named by what recoginible by 'Matcher')
// - count: count of workers to be initiated
// returns:
// - id: identifier of this group of workers
// - remain: count of workers remain not initiated

// - err: any error
func (me *_workers) allocate(m Matcher, fn interface{}, count int) (id string, remain int, err error) {
	// make sure type of fn is relfect.Func
	k := reflect.TypeOf(fn).Kind()
	if k != reflect.Func {
		err = errors.New(fmt.Sprintf("Invalid function pointer passed: %v", k.String()))
		return
	}

	// make sure a valid 'Matcher' instance is assigned
	if m == nil {
		err = errors.New(fmt.Sprintf("Need a valid Matcher, not %v", m))
		return
	}

	if me.workers == nil {
		err = errors.New("worker slice is not initialized")
		return
	}

	var (
		w   *worker
		eid int
	)
	defer func() {
		if err != nil {
			if eid != 0 {
				_, err_ := me.eventMux.Unregister(eid)
				if err_ != nil {
					// TODO: log it
				}
			}

			if w != nil {
				err_ := w.rs.Close()
				if err_ != nil {
					// TODO: log it?
				}
				close(w.tasks)
			}
		} else {
			remain, err = me.more(id, count)
		}
	}()

	err = func() (err error) {
		me.lock.Lock()
		defer me.lock.Unlock()

		// get an unique id
		for {
			id = uuid.NewV4().String()
			_, ok := me.workers[id]
			if !ok {
				break
			}
		}

		// initiate controlling channle
		w = &worker{
			matcher: m,
			tasks:   make(chan meta.Task, 10), // TODO: configuration?
			rs:      common.NewRoutines(),
			fn:      fn,
		}

		eid, err = me.eventMux.Register(w.rs.Events(), 1)
		if err != nil {
			return
		}

		me.workers[id] = w
		return
	}()

	return
}

// allocating more workers
//
// parameters:
// - id: identifier of this group of workers share the same function, matcher...
// - count: count of workers to be initiated
// returns:
// - remain: count of workers remain not initiated
// - err: any error
func (me *_workers) more(id string, count int) (remain int, err error) {
	// locking
	me.lock.Lock()
	defer me.lock.Unlock()

	if count < 0 {
		err = errors.New(fmt.Sprintf("Negative count is provided %v", count))
	}
	remain = count

	if me.workers == nil {
		err = errors.New("worker slice is not initialized")
		return
	}

	if me.reports == nil {
		err = errors.New("report channel is not set")
		return
	}

	// checking existence of Id
	w, ok := me.workers[id]
	if !ok {
		err = errors.New(fmt.Sprintf("%d group of worker not found"))
		return
	}

	// initiating workers
	for ; remain > 0; remain-- {
		go _worker_routine_(w.rs.New(), w.rs.Wait(), w.rs.Events(), w.tasks, me.reports, w.fn)
	}

	return
}

// dispatching a 'meta.Task'
//
// parameters:
// - t: the task
// returns:
// - err: any error
func (me *_workers) dispatch(t meta.Task) (err error) {
	me.lock.RLock()
	defer me.lock.RUnlock()

	// compose a report -- sent
	var (
		r    meta.Report
		err_ error
	)

	r, err = t.ComposeReport(meta.Status.Sent, nil, nil)
	if err != nil {
		r, err_ = t.ComposeReport(meta.Status.Fail, nil, meta.NewErr(0, err))
		if err_ != nil {
			// TODO: log it
		}
		return
	}
	me.reports <- r

	found := false
	if me.workers != nil {
		for _, v := range me.workers {
			if v.matcher.Match(t.GetName()) {
				v.tasks <- t
				found = true
				break
			}
		}
	}

	if found {
		return nil
	}
	return errWorkerNotFound
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
	me.lock.Lock()
	defer me.lock.Unlock()

	// stop all workers routine
	for _, v := range me.workers {
		v.rs.Close()
	}

	// unbind reports channel
	close(me.reports)
	me.reports = make(chan meta.Report, 10)

	return
}

func (me *_workers) reportsChannel() <-chan meta.Report {
	return me.reports
}

// factory function
func newWorkers() (w *_workers, err error) {
	w = &_workers{
		workers:        make(map[string]*worker),
		reports:        make(chan meta.Report, 10),
		eventConverter: common.NewRoutines(),
		events:         make(chan *common.Event, 10),
		eventMux:       common.NewMux(),
	}

	go func(quit <-chan int, wait *sync.WaitGroup, in <-chan *common.MuxOut, out chan *common.Event) {
		defer wait.Done()
		for {
			select {
			case _, _ = <-quit:
				goto cleanup
			case v, ok := <-in:
				fmt.Println("receiving event in worker")
				if !ok {
					goto cleanup
				}
				out <- v.Value.(*common.Event)
			}
		}
	cleanup:
	}(w.eventConverter.New(), w.eventConverter.Wait(), w.eventMux.Out(), w.events)

	remain, err := w.eventMux.More(1)
	if err == nil && remain != 0 {
		err = errors.New(fmt.Sprintf("Unable to allocate mux routine:%v"))
	}

	return
}

//
// worker routine
//

func _worker_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event, tasks <-chan meta.Task, reports chan<- meta.Report, fn interface{}) {
	defer wait.Done()
	// TODO: concider a shared, common invoker instance?
	ivk := meta.NewDefaultInvoker()

	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				goto cleanup
			}

			var (
				r         meta.Report
				ret       []interface{}
				err, err_ error
				status    int
			)

			// compose a report -- progress
			r, err = t.ComposeReport(meta.Status.Progress, nil, nil)
			if err != nil {
				// TODO: a test case here
				r, err_ = t.ComposeReport(meta.Status.Fail, nil, meta.NewErr(0, err))
				if err_ != nil {
					events <- common.NewEventFromError(common.InstT.WORKER, err_)
					break
				}
				reports <- r
				break
			}
			reports <- r

			// call the actuall function, where is the magic
			ret, err = ivk.Invoke(fn, t.GetArgs())

			// compose a report -- done / fail
			if err != nil {
				status = meta.Status.Fail
				events <- common.NewEventFromError(common.InstT.WORKER, err_)
			} else {
				status = meta.Status.Done
			}

			r, err_ = t.ComposeReport(status, ret, meta.NewErr(0, err))
			if err_ != nil {
				events <- common.NewEventFromError(common.InstT.WORKER, err_)
				break
			}
			reports <- r

		case _, _ = <-quit:
			// nothing to clean
			goto cleanup
		}
	}
cleanup:
}
