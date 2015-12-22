package dingo

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type localStorePoller struct {
	task    *Task
	reports chan<- *Report
}

type localBridge struct {
	objLock   sync.RWMutex
	needed    int
	broker    chan *Task
	listeners *Routines
	reporters *Routines
	pollers   chan *localStorePoller
	events    chan *Event
	eventMux  *mux
}

func newLocalBridge(args ...interface{}) (b bridge) {
	v := &localBridge{
		events:    make(chan *Event, 10),
		eventMux:  newMux(),
		listeners: NewRoutines(),
		reporters: NewRoutines(),
		broker:    make(chan *Task, 10),
		pollers:   make(chan *localStorePoller, 10),
	}
	b = v

	v.eventMux.Handle(func(val interface{}, _ int) {
		v.events <- val.(*Event)
	})

	return
}

func (me *localBridge) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		me.events,
	}, nil
}

func (me *localBridge) Close() (err error) {
	me.objLock.Lock()
	defer me.objLock.Unlock()

	me.listeners.Close()
	me.reporters.Close()
	me.eventMux.Close()

	close(me.broker)
	me.broker = make(chan *Task, 10)
	return
}

func (me *localBridge) Register(name string, fn interface{}) (err error) {
	return
}

func (me *localBridge) SendTask(t *Task) (err error) {
	me.objLock.RLock()
	defer me.objLock.RUnlock()

	if me.needed&ObjT.PRODUCER == 0 {
		err = errors.New("producer is not attached")
		return
	}

	me.broker <- t
	return
}

func (me *localBridge) AddNamedListener(name string, rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	err = errors.New("named consumer is not supported by local-bridge")
	return
}

func (me *localBridge) AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	me.objLock.RLock()
	defer me.objLock.RUnlock()

	tasks2 := make(chan *Task, 10)
	tasks = tasks2

	if me.needed&ObjT.CONSUMER == 0 {
		err = errors.New("consumer is not attached")
		return
	}

	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *Event,
		input <-chan *Task,
		output chan<- *Task,
		receipts <-chan *TaskReceipt,
	) {
		defer wait.Done()
		out := func(t *Task) (done bool) {
			output <- t
			reply, ok := <-receipts
			if !ok {
				done = true
				return
			}
			if reply.ID != t.ID() {
				events <- NewEventFromError(
					ObjT.CONSUMER,
					errors.New(fmt.Sprintf("expect receipt from %v, but %v", t, reply)),
				)
				return
			}
			if reply.Status == ReceiptStatus.WORKER_NOT_FOUND {
				events <- NewEventFromError(
					ObjT.CONSUMER,
					errors.New(fmt.Sprintf("workers not found: %v", t)),
				)
				return
			}

			return
		}
		for {
			select {
			case _, _ = <-quit:
				goto clean
			case t, ok := <-input:
				if !ok {
					goto clean
				}
				if out(t) {
					goto clean
				}
			}
		}
	clean:
		finished := false
		for {
			select {
			case t, ok := <-input:
				if !ok {
					finished = true
					break
				}
				if out(t) {
					finished = true
				}
			default:
				finished = true
			}
			if finished {
				break
			}
		}
		close(output)
	}(me.listeners.New(), me.listeners.Wait(), me.listeners.Events(), me.broker, tasks2, rcpt)

	return
}

func (me *localBridge) StopAllListeners() (err error) {
	me.objLock.Lock()
	defer me.objLock.Unlock()

	if me.needed&ObjT.CONSUMER == 0 {
		err = errors.New("consumer is not attached")
		return
	}

	me.listeners.Close()
	return
}

func (me *localBridge) Report(reports <-chan *Report) (err error) {
	me.objLock.RLock()
	defer me.objLock.RUnlock()

	if me.needed&ObjT.REPORTER == 0 {
		err = errors.New("reporter is not attached")
		return
	}

	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *Event,
		inputs <-chan *Report,
		pollers chan *localStorePoller,
	) {
		// each time Report is called, a dedicated 'watch', 'unSent' is allocated,
		// they are natually thread-safe (used in one go routine only)
		var (
			// map (name, id) to poller
			watched map[string]map[string]*localStorePoller = make(map[string]map[string]*localStorePoller)

			// map (name, id) to slice of unsent reports.
			unSent map[string]map[string][]*Report = make(map[string]map[string][]*Report)

			id, name string
			poller   *localStorePoller
		)

		defer wait.Done()
		outF := func(r *Report) (found bool) {
			id, name = r.ID(), r.Name()
			if ids, ok := watched[name]; ok {
				if poller, found = ids[id]; found {
					poller.reports <- r
					if r.Done() {
						delete(ids, id)
						close(poller.reports)
					}
				}
			}

			return
		}

		for {
			select {
			case _, _ = <-quit:
				goto clean
			case p, ok := <-pollers:
				if !ok {
					goto clean
				}
				id, name = p.task.ID(), p.task.Name()

				if ids, ok := watched[name]; ok {
					if _, ok := ids[id]; ok {
						events <- NewEventFromError(
							ObjT.STORE,
							errors.New(fmt.Sprintf("duplicated polling found: %v", id)),
						)
						break
					}
				}

				// those reports would only be settle down when some
				// reports coming in.
				if ids, ok := unSent[name]; ok {
					if unst, ok := ids[id]; ok {
						if w, ok := watched[name]; ok {
							w[id] = p
						} else {
							watched[name] = map[string]*localStorePoller{id: p}
						}
						for _, u := range unst {
							outF(u)
						}
						delete(ids, id)
						break
					}
				}

				// if the other ends forgets to send any report', this poller might be
				// traveled in pollers channelf forever.
				pollers <- p

				// avoid busy looping
				<-time.After(100 * time.Millisecond)

			case v, ok := <-inputs:
				if !ok {
					goto clean
				}

				if outF(v) {
					break
				}

				id, name = v.ID(), v.Name()
				// store it in un-sent array
				if rs, ok := unSent[name]; ok {
					if unSentReports, ok := rs[id]; ok {
						rs[id] = append(unSentReports, v)
					} else {
						rs[id] = []*Report{v}
					}
				} else {
					unSent[name] = map[string][]*Report{id: []*Report{v}}
				}
			}
		}
	clean:
		finished := false
		for {
			select {
			case v, ok := <-inputs:
				if !ok {
					finished = true
					break
				}

				if !outF(v) {
					events <- NewEventFromError(
						ObjT.STORE,
						errors.New(fmt.Sprintf("droping report: %v", v)),
					)
				}
			default:
				finished = true
			}

			if finished {
				break
			}
		}

		for k, v := range watched {
			for kk, vv := range v {
				events <- NewEventFromError(
					ObjT.STORE,
					errors.New(fmt.Sprintf("unclosed reports channel: %v:%v", k, kk)),
				)

				// send a 'Shutdown' report
				r, err := vv.task.composeReport(Status.Fail, nil, NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
				if err != nil {
					events <- NewEventFromError(ObjT.STORE, err)
				} else {
					vv.reports <- r
				}

				// remember to send t close signal
				close(vv.reports)
			}
		}
		for _, v := range unSent {
			for _, vv := range v {
				for _, r := range vv {
					events <- NewEventFromError(
						ObjT.STORE,
						errors.New(fmt.Sprintf("unsent report: %v", r)),
					)
				}
			}
		}
	}(me.reporters.New(), me.reporters.Wait(), me.reporters.Events(), reports, me.pollers)

	return
}

func (me *localBridge) Poll(t *Task) (reports <-chan *Report, err error) {
	if me.needed&ObjT.STORE == 0 {
		err = errors.New("store is not attached")
		return
	}
	reports2 := make(chan *Report, Status.Count)
	me.pollers <- &localStorePoller{
		task:    t,
		reports: reports2,
	}

	reports = reports2
	return
}

func (me *localBridge) AttachReporter(r Reporter) (err error) {
	me.needed |= ObjT.REPORTER
	return
}

func (me *localBridge) AttachStore(s Store) (err error) {
	me.needed |= ObjT.STORE
	return
}

func (me *localBridge) AttachProducer(p Producer) (err error) {
	me.needed |= ObjT.PRODUCER
	return
}

func (me *localBridge) AttachConsumer(c Consumer, nc NamedConsumer) (err error) {
	me.needed |= ObjT.CONSUMER
	return
}

func (me *localBridge) Exists(it int) bool {
	// make sure only one component is selected
	switch it {
	case ObjT.PRODUCER:
		return me.needed&it == it
	case ObjT.CONSUMER:
		return me.needed&it == it
	case ObjT.REPORTER:
		return me.needed&it == it
	case ObjT.STORE:
		return me.needed&it == it
	}

	return false
}

func (me *localBridge) ReporterHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (me *localBridge) ProducerHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}
