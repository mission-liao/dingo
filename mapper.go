package dingo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

//
// mapper container
//

type _mappers struct {
	workers *_workers
	mappers *Routines
	toLock  sync.Mutex
	to      atomic.Value
}

// allocating more mappers
//
// parameters:
// - tasks: input channel for Task
// - receipts: output channel for TaskReceipt
func (me *_mappers) more(tasks <-chan *Task, receipts chan<- *TaskReceipt) {
	go me._mapper_routine_(me.mappers.New(), me.mappers.Wait(), me.mappers.Events(), tasks, receipts)
}

// dispatching a 'Task'
//
// parameters:
// - t: the task
// returns:
// - err: any error
func (me *_mappers) dispatch(t *Task) (err error) {
	all := me.to.Load().(map[string]chan *Task)
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

func (me *_mappers) allocateWorkers(name string, count, share int) ([]<-chan *Report, int, error) {
	me.toLock.Lock()
	defer me.toLock.Unlock()

	all := me.to.Load().(map[string]chan *Task)
	if _, ok := all[name]; ok {
		return nil, count, errors.New(fmt.Sprintf("already registered: %v", name))
	}
	t := make(chan *Task, 10)
	r, n, err := me.workers.allocate(name, t, nil, count, share)
	if err != nil {
		return r, n, err
	}

	alln := make(map[string]chan *Task)
	for k := range all {
		alln[k] = all[k]
	}
	alln[name] = t
	me.to.Store(alln)
	return r, n, err
}

//
// Object interface
//

func (me *_mappers) Events() (ret []<-chan *Event, err error) {
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

	all := m.to.Load().(map[string]chan *Task)
	for _, v := range all {
		close(v)
	}
	m.to.Store(make(map[string]chan *Task))

	return
}

// factory function
// parameters:
// - tasks: input channel
// returns:
// ...
func newMappers(trans *mgr, hooks exHooks) (m *_mappers, err error) {
	w, err := newWorkers(trans, hooks)
	if err != nil {
		return
	}

	m = &_mappers{
		workers: w,
		mappers: NewRoutines(),
	}

	m.to.Store(make(map[string]chan *Task))
	return
}

//
// mapper routine
//

func (m *_mappers) _mapper_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *Event,
	tasks <-chan *Task,
	receipts chan<- *TaskReceipt,
) {
	defer wait.Done()
	defer close(receipts)

	receive := func(t *Task) {
		// find registered worker
		err := m.dispatch(t)

		// compose a receipt
		var rpt TaskReceipt
		if err != nil {
			// send an error event
			events <- NewEventFromError(ObjT.MAPPER, err)

			if err == errWorkerNotFound {
				rpt = TaskReceipt{
					Status: ReceiptStatus.WORKER_NOT_FOUND,
				}
			} else {
				rpt = TaskReceipt{
					Status:  ReceiptStatus.NOK,
					Payload: err,
				}
			}
		} else {
			rpt = TaskReceipt{
				Status: ReceiptStatus.OK,
			}
		}
		receipts <- &rpt
	}

finished:
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				break finished
			}
			receive(t)

		case <-quit:
			// clean up code below
			break finished
		}
	}

done:
	// consuming remaining tasks in channel.
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				break done
			}
			receive(t)
		default:
			break done
		}
	}
	return
}
