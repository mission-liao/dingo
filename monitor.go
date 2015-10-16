package dingo

import (
	"errors"
	"sync"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/task"
)

type _watch struct {
	last    int
	reports chan task.Report
}

//
// container of monitor
//
type _monitors struct {
	mntlock   sync.Mutex
	monitors  []*monitor
	watchLock sync.RWMutex
	watched   map[string]*_watch
	reports   <-chan task.Report
	store     backend.Store
}

func (me *_monitors) init() (err error) {
	if me.store == nil {
		err = errors.New("store is not assigned.")
		return
	}

	me.reports, err = me.store.Subscribe()
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
	mts := make([]*monitor, 0, count)
	remain = count

	for ; remain > 0; remain-- {
		mt := &monitor{
			common.RtControl{
				Quit: make(chan int, 1),
				Done: make(chan int, 1),
			},
		}
		mts = append(mts, mt)
		go me._monitor_routine_(mt.Quit, mt.Done)
	}

	me.mntlock.Lock()
	defer me.mntlock.Unlock()

	me.monitors = append(me.monitors, mts...)
	return
}

//
func (me *_monitors) done() (err error) {
	me.mntlock.Lock()
	defer me.mntlock.Unlock()

	// stop all monitors routine
	for _, v := range me.monitors {
		v.Close()
	}

	// TODO: close all output channels

	me.reports = nil
	me.monitors = nil
	return
}

func (me *_monitors) check(t task.Task) (reports <-chan task.Report, err error) {
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
				last:    task.Status.None,
				reports: make(chan task.Report, task.Status.Count),
			}
			reports = w.reports
			me.watched[t.GetId()] = w
		} else {
			reports = w.reports
		}
	}()

	return
}

//
// record for a monitor
//

type monitor struct {
	common.RtControl
}

// factory function
//
// paramters:
// - reports: input channel
func newMonitors(store backend.Store) (mnt *_monitors, err error) {
	mnt = &_monitors{
		store:    store,
		monitors: make([]*monitor, 0, 10),
		watched:  make(map[string]*_watch),
	}
	err = mnt.init()
	return
}

// monitor routine
//
func (me *_monitors) _monitor_routine_(quit <-chan int, done chan<- int) {
	for {
		select {
		case report, ok := <-me.reports:
			if !ok {
				goto cleanup
			}
			_2delete := false
			func() {
				me.watchLock.RLock()
				defer me.watchLock.RUnlock()

				w, ok := me.watched[report.GetId()]
				if !ok {
					// TODO: log it

					// discard this report,
					// something wrong in backend
					return
				}

				if w.last == report.GetStatus() {
					// duplicated report, discard it
					return
				} else if report.Done() {
					_ = me.store.Done(report)
					// TODO: a channel to report errors

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
						return
					}
					delete(me.watched, id)
					close(w.reports)
				}()
			}
		case _, _ = <-quit:
			goto cleanup
		}
	}

cleanup:
	done <- 1
	return
}
