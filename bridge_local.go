package dingo

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

type poller struct {
	id      string
	reports chan<- *transport.Report
}

type localBridge struct {
	objLock   sync.RWMutex
	needed    int
	broker    chan *transport.Task
	listeners *common.Routines
	reporters *common.Routines
	pollers   chan *poller
	events    chan *common.Event
	eventMux  *common.Mux
}

func newLocalBridge(args ...interface{}) (b *localBridge) {
	b = &localBridge{
		events:    make(chan *common.Event, 10),
		eventMux:  common.NewMux(),
		listeners: common.NewRoutines(),
		reporters: common.NewRoutines(),
		broker:    make(chan *transport.Task, 10),
		pollers:   make(chan *poller, 10),
	}

	b.eventMux.Handle(func(val interface{}, _ int) {
		b.events <- val.(*common.Event)
	})

	return
}

func (me *localBridge) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.events,
	}, nil
}

func (me *localBridge) Close() (err error) {
	me.objLock.Lock()
	defer me.objLock.Unlock()

	me.listeners.Close()
	me.eventMux.Close()

	close(me.broker)
	me.broker = make(chan *transport.Task, 10)
	return
}

func (me *localBridge) Register(name string, fn interface{}) (err error) {
	return
}

func (me *localBridge) SendTask(t *transport.Task) (err error) {
	me.objLock.RLock()
	defer me.objLock.RUnlock()

	if me.needed&common.InstT.PRODUCER == 0 {
		err = errors.New("producer is not attached")
		return
	}

	me.broker <- t
	return
}

func (me *localBridge) AddNamedListener(name string, rcpt <-chan *broker.Receipt) (tasks <-chan *transport.Task, err error) {
	err = errors.New("named consumer is not supported by local-bridge")
	return
}

func (me *localBridge) AddListener(rcpt <-chan *broker.Receipt) (tasks <-chan *transport.Task, err error) {
	me.objLock.RLock()
	defer me.objLock.RUnlock()

	tasks2 := make(chan *transport.Task, 10)
	tasks = tasks2

	if me.needed&common.InstT.CONSUMER == 0 {
		err = errors.New("consumer is not attached")
		return
	}

	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *common.Event,
		input <-chan *transport.Task,
		output chan<- *transport.Task,
		receipts <-chan *broker.Receipt,
	) {
		defer wait.Done()
		out := func(t *transport.Task) (done bool) {
			output <- t
			reply, ok := <-receipts
			if !ok {
				done = true
				return
			}
			if reply.ID != t.ID() {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("expect receipt from %v, but %v", t, reply)),
				)
				return
			}
			if reply.Status == broker.Status.WORKER_NOT_FOUND {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
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

	if me.needed&common.InstT.CONSUMER == 0 {
		err = errors.New("consumer is not attached")
		return
	}

	err = me.listeners.Close()
	return
}

func (me *localBridge) Report(reports <-chan *transport.Report) (err error) {
	me.objLock.RLock()
	defer me.objLock.RUnlock()

	if me.needed&common.InstT.REPORTER == 0 {
		err = errors.New("reporter is not attached")
		return
	}

	var (
		watched map[string]chan<- *transport.Report = make(map[string]chan<- *transport.Report)
		unSent  map[string][]*transport.Report      = make(map[string][]*transport.Report)
	)

	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *common.Event,
		inputs <-chan *transport.Report,
		pollers chan *poller,
	) {
		defer wait.Done()
		outF := func(r *transport.Report) (found bool) {
			o, found := watched[r.ID()]
			if found {
				// TODO: fix returns here
				o <- r
				if r.Done() {
					delete(watched, r.ID())
					close(o)
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

				if _, ok := watched[p.id]; ok {
					events <- common.NewEventFromError(
						common.InstT.STORE,
						errors.New(fmt.Sprintf("duplicated polling found: %v", p.id)),
					)
					break
				}

				if u, ok := unSent[p.id]; ok {
					watched[p.id] = p.reports
					for _, unst := range u {
						outF(unst)
					}
					break
				}

				pollers <- p
				<-time.After(100 * time.Millisecond) // avoid busy looping
			case v, ok := <-inputs:
				if !ok {
					goto clean
				}

				if outF(v) {
					break
				}

				// store it in un-sent array
				rs, ok := unSent[v.ID()]
				if !ok {
					unSent[v.ID()] = []*transport.Report{v}
				} else {
					unSent[v.ID()] = append(rs, v)
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
					events <- common.NewEventFromError(
						common.InstT.STORE,
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

		for k := range watched {
			events <- common.NewEventFromError(
				common.InstT.STORE,
				errors.New(fmt.Sprintf("unclosed reports channel: %v", k)),
			)
		}
		for _, v := range unSent {
			for _, r := range v {
				events <- common.NewEventFromError(
					common.InstT.STORE,
					errors.New(fmt.Sprintf("unsent report: %v", r)),
				)
			}
		}
	}(me.reporters.New(), me.reporters.Wait(), me.reporters.Events(), reports, me.pollers)

	return
}

func (me *localBridge) Poll(t *transport.Task) (reports <-chan *transport.Report, err error) {
	if me.needed&common.InstT.STORE == 0 {
		err = errors.New("store is not attached")
		return
	}
	reports2 := make(chan *transport.Report, transport.Status.Count)
	me.pollers <- &poller{
		id:      t.ID(),
		reports: reports2,
	}

	reports = reports2
	return
}

func (me *localBridge) AttachReporter(r backend.Reporter) (err error) {
	me.needed |= common.InstT.REPORTER
	return
}

func (me *localBridge) AttachStore(s backend.Store) (err error) {
	me.needed |= common.InstT.STORE
	return
}

func (me *localBridge) AttachProducer(p broker.Producer) (err error) {
	me.needed |= common.InstT.PRODUCER
	return
}

func (me *localBridge) AttachConsumer(c broker.Consumer, nc broker.NamedConsumer) (err error) {
	me.needed |= common.InstT.CONSUMER
	return
}

func (me *localBridge) Exists(it int) bool {
	// make sure only one component is selected
	switch it {
	case common.InstT.PRODUCER:
		return me.needed&it == it
	case common.InstT.CONSUMER:
		return me.needed&it == it
	case common.InstT.REPORTER:
		return me.needed&it == it
	case common.InstT.STORE:
		return me.needed&it == it
	}

	return false
}
