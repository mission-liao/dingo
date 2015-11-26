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
// mapper container
//

type _mappers struct {
	workers *_workers
	mappers *common.Routines
	toLock  sync.Mutex
	to      atomic.Value
}

// allocating more mappers
//
// parameters:
// - tasks: input channel for transport.Task
// - receipts: output channel for broker.Receipt
func (me *_mappers) more(tasks <-chan *transport.Task, receipts chan<- *broker.Receipt) {
	go me._mapper_routine_(me.mappers.New(), me.mappers.Wait(), me.mappers.Events(), tasks, receipts)
}

// dispatching a 'transport.Task'
//
// parameters:
// - t: the task
// returns:
// - err: any error
func (me *_mappers) dispatch(t *transport.Task) (err error) {
	all := me.to.Load().(map[string]chan *transport.Task)
	if out, ok := all[t.Name()]; ok {
		out <- t
	} else {
		err = errWorkerNotFound
	}
	return
}

//
// proxy of _workers
//

func (me *_mappers) allocateWorkers(name string, count, share int) ([]<-chan *transport.Report, int, error) {
	me.toLock.Lock()
	defer me.toLock.Unlock()

	all := me.to.Load().(map[string]chan *transport.Task)
	if _, ok := all[name]; ok {
		return nil, count, errors.New(fmt.Sprintf("already registered: %v", name))
	}
	t := make(chan *transport.Task, 10)
	r, n, err := me.workers.allocate(name, t, nil, count, share)
	if err != nil {
		return r, n, err
	}

	alln := make(map[string]chan *transport.Task)
	for k := range all {
		alln[k] = all[k]
	}
	alln[name] = t
	me.to.Store(alln)
	return r, n, err
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

	m.toLock.Lock()
	defer m.toLock.Unlock()

	all := m.to.Load().(map[string]chan *transport.Task)
	for _, v := range all {
		close(v)
	}
	m.to.Store(make(map[string]chan *transport.Task))

	return
}

// factory function
// parameters:
// - tasks: input channel
// returns:
// ...
func newMappers(trans *transport.Mgr) (m *_mappers, err error) {
	w, err := newWorkers(trans)
	if err != nil {
		return
	}

	m = &_mappers{
		workers: w,
		mappers: common.NewRoutines(),
	}

	m.to.Store(make(map[string]chan *transport.Task))
	return
}

//
// mapper routine
//

func (m *_mappers) _mapper_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *common.Event,
	tasks <-chan *transport.Task,
	receipts chan<- *broker.Receipt,
) {
	defer wait.Done()
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				// TODO: channel is closed
				goto cleanup
			}

			// find registered worker
			err := m.dispatch(t)

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
			receipts <- &rpt

		case <-quit:
			// clean up code below
			goto cleanup
		}
	}
cleanup:
	// TODO: cleanup
	close(receipts)
	return
}
