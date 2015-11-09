package dingo

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

type _watch struct {
	last    int
	reports chan meta.Report
}

type _fn struct {
	m  Matcher
	fn interface{}
}

//
// container of monitor
//
type _monitors struct {
	monitors  *common.Routines
	watchLock sync.RWMutex
	watched   map[string]*_watch
	reports   <-chan meta.Report
	store     backend.Store
	fnLock    sync.RWMutex
	fns       []*_fn
	invoker   meta.Invoker
}

// factory function
//
// paramters:
// - reports: input channel
func newMonitors(store backend.Store) (mnt *_monitors, err error) {
	mnt = &_monitors{
		store:    store,
		monitors: common.NewRoutines(),
		watched:  make(map[string]*_watch),
		fns:      make([]*_fn, 0, 10),
		invoker:  meta.NewDefaultInvoker(),
	}

	if mnt.store == nil {
		err = errors.New("store is not assigned.")
		return
	}

	mnt.reports, err = mnt.store.Subscribe()
	if err != nil {
		return
	}

	if mnt.reports == nil {
		err = errors.New("errs/reports channel is not available")
		return
	}

	return
}

//
func (me *_monitors) register(m Matcher, fn interface{}) (err error) {
	me.fnLock.Lock()
	defer me.fnLock.Unlock()

	me.fns = append(me.fns, &_fn{
		m:  m,
		fn: fn,
	})

	return
}

// allocating more monitors
//
// parameters:
// - count: count of monitors to be allocated
// returns:
// - remains: count of un-allocated monitors
// - err: any error
func (me *_monitors) more(count int) (remain int, err error) {
	remain = count
	for ; remain > 0; remain-- {
		quit := me.monitors.New()
		go me._monitor_routine_(quit, me.monitors.Wait(), me.monitors.Events())
	}
	return
}

//
// common.Object interface
//

func (me *_monitors) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.monitors.Events(),
	}, nil
}

func (me *_monitors) Close() (err error) {
	me.monitors.Close()

	// TODO: close all output channels
	return
}

func (me *_monitors) check(t meta.Task) (reports <-chan meta.Report, err error) {
	err = me.store.Poll(t)
	if err != nil {
		return
	}

	func() {
		me.watchLock.Lock()
		defer me.watchLock.Unlock()

		w, ok := me.watched[t.GetId()]
		if !ok {
			// make sure this channel has enough
			// buffer size

			w := &_watch{
				last:    meta.Status.None,
				reports: make(chan meta.Report, meta.Status.Count),
			}
			reports = w.reports
			me.watched[t.GetId()] = w
		} else {
			reports = w.reports
		}
	}()

	return
}

// monitor routine
//
func (me *_monitors) _monitor_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event) {
	defer wait.Done()
	for {
		select {
		case report, ok := <-me.reports:
			if !ok {
				return
			}

			// convert returns to right type
			func() {
				returns := report.GetReturn()
				if returns == nil || len(returns) == 0 {
					return
				}

				me.fnLock.RLock()
				defer me.fnLock.RUnlock()
				for _, v := range me.fns {
					if !v.m.Match(report.GetName()) {
						continue
					}

					returns, err := me.invoker.Return(v.fn, returns)
					if err != nil {
						events <- common.NewEventFromError(common.InstT.MONITOR, err)
						continue
					}

					report.SetReturn(returns)
				}
			}()

			_2delete := false
			func() {
				me.watchLock.RLock()
				defer me.watchLock.RUnlock()

				w, ok := me.watched[report.GetId()]
				if !ok {
					events <- common.NewEventFromError(
						common.InstT.MONITOR,
						errors.New(fmt.Sprintf("ID not found:%v", report)),
					)

					// discard this report,
					// something wrong in backend
					return
				}

				// TODO: allow multiple failure report?
				if w.last == report.GetStatus() {
					events <- common.NewEventFromError(
						common.InstT.MONITOR,
						errors.New(fmt.Sprintf("report duplication:%v", report)),
					)

					// duplicated report, discard it
					return
				} else if report.Done() {
					err := me.store.Done(report)
					if err != nil {
						events <- common.NewEventFromError(common.InstT.MONITOR, err)
					}

					_2delete = true
				}

				// this line should never be blocked.
				//
				// we should always allocate enough buffer
				// for all possible reports.
				w.reports <- report
			}()

			if _2delete {
				func() {
					// need writer lock this time
					me.watchLock.Lock()
					defer me.watchLock.Unlock()

					id := report.GetId()
					w, ok := me.watched[id]
					if !ok {
						events <- common.NewEventFromError(
							common.InstT.MONITOR,
							errors.New(fmt.Sprintf("when deletion, ID not found:%v", report)),
						)

						return
					}
					delete(me.watched, id)
					close(w.reports)
				}()
			}
		case _, _ = <-quit:
			return
		}
	}
}
