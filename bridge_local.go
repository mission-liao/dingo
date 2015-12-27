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
	supported int
	broker    chan *Task
	listeners *Routines
	reporters *Routines
	pollers   chan *localStorePoller
	events    chan *Event
}

func newLocalBridge(args ...interface{}) (b bridge) {
	v := &localBridge{
		events:    make(chan *Event, 10),
		listeners: NewRoutines(),
		reporters: NewRoutines(),
		broker:    make(chan *Task, 10),
		pollers:   make(chan *localStorePoller, 10),
		supported: ObjT.Reporter | ObjT.Store | ObjT.Producer | ObjT.Consumer,
	}
	b = v

	return
}

func (bdg *localBridge) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		bdg.events,
	}, nil
}

func (bdg *localBridge) Close() (err error) {
	bdg.objLock.Lock()
	defer bdg.objLock.Unlock()

	bdg.listeners.Close()
	bdg.reporters.Close()

	close(bdg.broker)
	bdg.broker = make(chan *Task, 10)
	return
}

func (bdg *localBridge) SendTask(t *Task) (err error) {
	bdg.objLock.RLock()
	defer bdg.objLock.RUnlock()

	bdg.broker <- t
	return
}

func (bdg *localBridge) AddNamedListener(name string, rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	err = errors.New("named consumer is not supported by local-bridge")
	return
}

func (bdg *localBridge) AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	bdg.objLock.RLock()
	defer bdg.objLock.RUnlock()

	tasks2 := make(chan *Task, 10)
	tasks = tasks2

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
					ObjT.Consumer,
					fmt.Errorf("expect receipt from %v, but %v", t, reply),
				)
				return
			}
			if reply.Status == ReceiptStatus.WorkerNotFound {
				events <- NewEventFromError(
					ObjT.Consumer,
					fmt.Errorf("workers not found: %v", t),
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
		for {
			select {
			case t, ok := <-input:
				if !ok {
					break clean
				}
				if out(t) {
					break clean
				}
			default:
				break clean
			}
		}
		close(output)
	}(bdg.listeners.New(), bdg.listeners.Wait(), bdg.listeners.Events(), bdg.broker, tasks2, rcpt)

	return
}

func (bdg *localBridge) StopAllListeners() (err error) {
	bdg.objLock.Lock()
	defer bdg.objLock.Unlock()

	bdg.listeners.Close()
	return
}

func (bdg *localBridge) Report(reports <-chan *Report) (err error) {
	bdg.objLock.RLock()
	defer bdg.objLock.RUnlock()

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
			watched = make(map[string]map[string]*localStorePoller)

			// map (name, id) to slice of unsent reports.
			unSent = make(map[string]map[string][]*Report)

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
							ObjT.Store,
							fmt.Errorf("duplicated polling found: %v", id),
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
		for {
			select {
			case v, ok := <-inputs:
				if !ok {
					break clean
				}

				if !outF(v) {
					events <- NewEventFromError(
						ObjT.Store,
						fmt.Errorf("droping report: %v", v),
					)
				}
			default:
				break clean
			}
		}

		for k, v := range watched {
			for kk, vv := range v {
				events <- NewEventFromError(
					ObjT.Store,
					fmt.Errorf("unclosed reports channel: %v:%v", k, kk),
				)

				// send a 'Shutdown' report
				r, err := vv.task.composeReport(Status.Fail, nil, NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
				if err != nil {
					events <- NewEventFromError(ObjT.Store, err)
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
						ObjT.Store,
						fmt.Errorf("unsent report: %v", r),
					)
				}
			}
		}
	}(bdg.reporters.New(), bdg.reporters.Wait(), bdg.reporters.Events(), reports, bdg.pollers)

	return
}

func (bdg *localBridge) Poll(t *Task) (reports <-chan *Report, err error) {
	reports2 := make(chan *Report, Status.Count)
	bdg.pollers <- &localStorePoller{
		task:    t,
		reports: reports2,
	}

	reports = reports2
	return
}

func (bdg *localBridge) AttachReporter(r Reporter) (err error) {
	return
}

func (bdg *localBridge) AttachStore(s Store) (err error) {
	return
}

func (bdg *localBridge) AttachProducer(p Producer) (err error) {
	return
}

func (bdg *localBridge) AttachConsumer(c Consumer, nc NamedConsumer) (err error) {
	return
}

func (bdg *localBridge) Exists(it int) bool {
	// make sure only one component is selected
	switch it {
	case ObjT.Producer:
		return bdg.supported&it == it
	case ObjT.Consumer:
		return bdg.supported&it == it
	case ObjT.Reporter:
		return bdg.supported&it == it
	case ObjT.Store:
		return bdg.supported&it == it
	}

	return false
}

func (bdg *localBridge) ReporterHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (bdg *localBridge) StoreHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (bdg *localBridge) ProducerHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (bdg *localBridge) ConsumerHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}
