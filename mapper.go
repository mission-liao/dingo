package dingo

import (
	"sync"

	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

//
// mapper container
//

type _mappers struct {
	workers *_workers
	mappers *common.Routines
}

// allocating more mappers
//
// parameters:
// - tasks: input channel for meta.Task
// - receipts: output channel for broker.Receipt
func (m *_mappers) more(tasks <-chan meta.Task, receipts chan<- broker.Receipt) {
	quit := m.mappers.New()
	go m._mapper_routine_(quit, m.mappers.Wait(), m.mappers.Events(), tasks, receipts)
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

func (me *_mappers) reports() <-chan meta.Report {
	return me.workers.reportsChannel()
}

//
// common.Object interface
//

func (me *_mappers) Events() (ret []<-chan *common.Event, err error) {
	ret, err = me.workers.Events()
	if err != nil {
		return
	}

	ret = append(ret, me.mappers.Events())
	return
}

func (m *_mappers) Close() (err error) {
	m.mappers.Close()
	err = m.workers.Close()
	return
}

// factory function
// parameters:
// - tasks: input channel
// returns:
// ...
func newMappers() (m *_mappers, err error) {
	w, err := newWorkers()
	if err != nil {
		return
	}

	m = &_mappers{
		workers: w,
		mappers: common.NewRoutines(),
	}
	return
}

//
// mapper routine
//

func (m *_mappers) _mapper_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event, tasks <-chan meta.Task, receipts chan<- broker.Receipt) {
	defer wait.Done()
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				// TODO: channel is closed
				goto cleanup
			}

			// find registered worker
			err := m.workers.dispatch(t)

			// compose a receipt
			var rpt broker.Receipt
			if err != nil {
				// send an error event
				events <- common.NewEventFromError(common.InstT.MAPPER, err)

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
			receipts <- rpt

		case <-quit:
			// clean up code below
			goto cleanup
		}
	}
cleanup:
	close(receipts)
	return
}
