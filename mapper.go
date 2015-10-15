package dingo

import (
	"sync"

	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/task"
)

//
// mapper container
//

type _mappers struct {
	workers  *_workers
	mappers  []*mapper
	lock     sync.RWMutex
	tasks    <-chan task.Task
	receipts chan<- broker.Receipt
}

// allocating more mappers
//
// parameters:
// - count: count of mappers to be allocated
// returns:
// - remains: count of un-allocated mappers
// - err: any error
func (m *_mappers) more(count int) (remain int, err error) {
	mps := make([]*mapper, 0, count)
	remain = count

	for ; remain > 0; remain-- {
		mp := &mapper{
			common.RtControl{
				Quit: make(chan int, 1),
				Done: make(chan int, 1),
			},
		}
		mps = append(mps, mp)
		go m._mapper_routine_(mp.Quit, mp.Done, m.tasks)
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.mappers = append(m.mappers, mps...)
	return
}

//
// proxy of _workers
//

func (me *_mappers) allocateWorkers(m Matcher, fn interface{}, count int) (string, int, error) {
	return me.workers.allocate(m, fn, count)
}

func (me *_mappers) moreWorkers(id string, count int) (int, error) {
	return me.workers.more(id, count)
}

func (me *_mappers) reportsChannel() <-chan task.Report {
	return me.workers.reportsChannel()
}

//
//
//
func (m *_mappers) done() (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// stop all mappers routine
	for _, v := range m.mappers {
		v.Close()
	}

	// clear mapper slice
	m.mappers = nil

	// unbound input channel
	m.tasks = nil

	return
}

// factory function
// parameters:
// - tasks: input channel
// returns:
// ...
func newMappers(tasks <-chan task.Task, receipts chan<- broker.Receipt) *_mappers {
	return &_mappers{
		workers:  newWorkers(),
		mappers:  make([]*mapper, 0, 10),
		tasks:    tasks,
		receipts: receipts,
	}
}

//
// record of mapper
//

type mapper struct {
	common.RtControl
}

//
// mapper routine
//

func (m *_mappers) _mapper_routine_(quit <-chan int, done chan<- int, tasks <-chan task.Task) {
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				// TODO: channel is closed
			}
			// find registered worker
			var err error

			func() {
				m.lock.RLock()
				defer m.lock.RUnlock()

				err = m.workers.dispatch(t)
			}()

			// compose a receipt
			var rpt broker.Receipt
			if err != nil {
				if err == errWorkerNotFound {
					rpt = broker.Receipt{
						Status: broker.Status.WORKER_NOT_FOUND,
					}
				} else {
					rpt = broker.Receipt{
						Status:  broker.Status.NOK,
						Payload: err,
					}
				}
			} else {
				rpt = broker.Receipt{
					Status: broker.Status.OK,
				}
			}
			m.receipts <- rpt

		case <-quit:
			// clean up code below
			done <- 1
			return
		}
	}
}
