package dingo

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/task"
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

type strMatcher struct {
	name string
}

func (me *strMatcher) Match(patt string) bool {
	return me.name == patt
}

func newStrMatcher(name string) Matcher {
	return &strMatcher{
		name: name,
	}
}

//
// worker
//
// a required record for a group of workers
//

type worker struct {
	matcher Matcher
	tasks   chan task.Task
	ctrls   []*common.RtControl
	fn      interface{}
}

//
// worker container
//

type _workers struct {
	lock    sync.RWMutex
	workers map[string]*worker
	reports chan task.Report
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

	func() {
		me.lock.Lock()
		defer me.lock.Unlock()

		if me.workers == nil {
			err = errors.New("worker slice is not initialized")
			return
		}

		// get an unique id
		for {
			id = uuid.NewV4().String()
			_, ok := me.workers[id]
			if !ok {
				break
			}
		}

		// initiate controlling channle
		ctrls := make([]*common.RtControl, 0, count)
		tasks := make(chan task.Task, 100) // TODO: configuration?

		me.workers[id] = &worker{
			matcher: m,
			tasks:   tasks,
			ctrls:   ctrls,
			fn:      fn,
		}
	}()

	remain, err = me.more(id, count)
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
		c := common.NewRtCtrl()
		w.ctrls = append(w.ctrls, c)

		go _worker_routine_(c.Quit, c.Done, w.tasks, me.reports, w.fn)
	}

	return
}

// dispatching a 'task.Task'
//
// parameters:
// - t: the task
// returns:
// - err: any error
func (me *_workers) dispatch(t task.Task) (err error) {
	me.lock.RLock()
	defer me.lock.RUnlock()

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
//
//
func (me *_workers) done() (err error) {
	me.lock.Lock()
	defer me.lock.Unlock()

	// stop all workers routine
	for _, v := range me.workers {
		for _, wk := range v.ctrls {
			wk.Close()
		}
	}

	// clear worker map
	me.workers = nil

	// unbind reports channel
	close(me.reports)
	me.reports = nil

	return
}

func (me *_workers) reportsChannel() <-chan task.Report {
	return me.reports
}

// factory function
func newWorkers() *_workers {
	return &_workers{
		workers: make(map[string]*worker),
		reports: make(chan task.Report, 10),
	}
}

//
// worker routine
//

func _worker_routine_(quit <-chan int, done chan<- int, tasks <-chan task.Task, reports chan<- task.Report, fn interface{}) {
	// TODO: concider a shared, common invoker instance?
	ivk := task.NewDefaultInvoker()

	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				// TODO: when channel is closed
			}
			var (
				r         task.Report
				ret       []interface{}
				err, err_ error
				status    int
			)

			// compose a report -- progress
			r, err = t.ComposeReport(task.Status.Progress, nil, nil)
			if err != nil {
				r, err_ = t.ComposeReport(task.Status.Fail, err, nil)
				if err_ != nil {
					// TODO: log it
				}
				break
			}
			reports <- r

			// call the actuall function, where is the magic
			ret, err = ivk.Invoke(fn, t.GetArgs())

			// compose a report -- done / fail
			if err != nil {
				status = task.Status.Fail
			} else {
				status = task.Status.Done
			}

			r, err_ = t.ComposeReport(status, err, ret)
			if err_ != nil {
				// TODO: log it
				break
			}
			reports <- r

		case _, _ = <-quit:
			// nothing to clean
			done <- 1
			return
		}
	}
}
