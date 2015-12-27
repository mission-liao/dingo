package dingo

import (
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
func (mp *_mappers) more(tasks <-chan *Task, receipts chan<- *TaskReceipt) {
	go mp.mapperRoutine(mp.mappers.New(), mp.mappers.Wait(), mp.mappers.Events(), tasks, receipts)
}

// dispatching a 'Task'
//
// parameters:
// - t: the task
// returns:
// - err: any error
func (mp *_mappers) dispatch(t *Task) (err error) {
	all := mp.to.Load().(map[string]chan *Task)
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

func (mp *_mappers) allocateWorkers(name string, count, share int) ([]<-chan *Report, int, error) {
	mp.toLock.Lock()
	defer mp.toLock.Unlock()

	all := mp.to.Load().(map[string]chan *Task)
	if _, ok := all[name]; ok {
		return nil, count, fmt.Errorf("already registered: %v", name)
	}
	t := make(chan *Task, 10)
	if r, n, err := mp.workers.allocate(name, t, nil, count, share); err != nil {
		return r, n, err
	} else {
		alln := make(map[string]chan *Task)
		for k := range all {
			alln[k] = all[k]
		}
		alln[name] = t
		mp.to.Store(alln)
		return r, n, err
	}
}

//
// Object interface
//

func (mp *_mappers) Expect(types int) (err error) {
	if types != ObjT.Mapper {
		err = fmt.Errorf("Unsupported types: %v", types)
		return
	}

	return
}

func (mp *_mappers) Events() (ret []<-chan *Event, err error) {
	if ret, err = mp.workers.Events(); err != nil {
		return
	}

	ret = append(ret, mp.mappers.Events())
	return
}

func (mp *_mappers) Close() (err error) {
	mp.mappers.Close()
	err = mp.workers.Close()

	mp.toLock.Lock()
	defer mp.toLock.Unlock()

	all := mp.to.Load().(map[string]chan *Task)
	for _, v := range all {
		close(v)
	}
	mp.to.Store(make(map[string]chan *Task))

	return
}

// factory function
// parameters:
// - tasks: input channel
// returns:
// ...
func newMappers(trans *fnMgr, hooks exHooks) (m *_mappers, err error) {
	var (
		w *_workers
	)
	if w, err = newWorkers(trans, hooks); err != nil {
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

func (mp *_mappers) mapperRoutine(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *Event,
	tasks <-chan *Task,
	receipts chan<- *TaskReceipt,
) {
	defer wait.Done()
	defer close(receipts)

	var (
		rpt *TaskReceipt
	)

	receive := func(t *Task) {
		// find registered worker
		if err := mp.dispatch(t); err != nil {
			// send an error event
			events <- NewEventFromError(ObjT.Mapper, err)

			if err == errWorkerNotFound {
				rpt = &TaskReceipt{
					ID:     t.ID(),
					Status: ReceiptStatus.WorkerNotFound,
				}
			} else {
				rpt = &TaskReceipt{
					ID:      t.ID(),
					Status:  ReceiptStatus.NOK,
					Payload: err,
				}
			}
		} else {
			rpt = &TaskReceipt{
				ID:     t.ID(),
				Status: ReceiptStatus.OK,
			}
		}
		receipts <- rpt
	}

	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				goto clean
			}
			receive(t)

		case <-quit:
			// clean up code below
			goto clean
		}
	}
clean:
	// consuming remaining tasks in channel.
	for {
		select {
		case t, ok := <-tasks:
			if !ok {
				break clean
			}
			receive(t)
		default:
			break clean
		}
	}
	return
}
