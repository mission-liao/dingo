package dingo

import (
	"errors"
	"sync"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

//
// default implementation
//

type defaultBridge struct {
	producerLock  sync.RWMutex
	producer      broker.Producer
	consumerLock  sync.RWMutex
	consumer      broker.Consumer
	namedConsumer broker.NamedConsumer
	reporterLock  sync.RWMutex
	reporter      backend.Reporter
	storeLock     sync.RWMutex
	store         backend.Store

	listeners *common.Routines
	reporters *common.Routines
	storers   *common.Routines
	doners    chan transport.Meta
	events    chan *common.Event
	eventMux  *common.Mux
	trans     *transport.Mgr
}

func newDefaultBridge(trans *transport.Mgr) (b *defaultBridge) {
	b = &defaultBridge{
		listeners: common.NewRoutines(),
		reporters: common.NewRoutines(),
		storers:   common.NewRoutines(),
		events:    make(chan *common.Event, 10),
		eventMux:  common.NewMux(),
		doners:    make(chan transport.Meta, 10),
		trans:     trans,
	}

	b.eventMux.Handle(func(val interface{}, _ int) {
		b.events <- val.(*common.Event)
	})

	return
}

func (me *defaultBridge) Close() (err error) {
	me.listeners.Close()
	me.reporters.Close()
	me.storers.Close()

	me.eventMux.Close()
	close(me.events)
	me.events = make(chan *common.Event, 10)

	return
}

func (me *defaultBridge) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.events,
	}, nil
}

func (me *defaultBridge) SendTask(t *transport.Task) (err error) {
	me.producerLock.RLock()
	defer me.producerLock.RUnlock()

	if me.producer == nil {
		err = errors.New("producer is not attached")
		return
	}

	b, err := me.trans.EncodeTask(t)
	if err != nil {
		return
	}

	err = me.producer.Send(t, b)
	return
}

func (me *defaultBridge) AddNamedListener(name string, receipts <-chan *broker.Receipt) (tasks <-chan *transport.Task, err error) {
	me.consumerLock.RLock()
	defer me.consumerLock.RUnlock()

	if me.namedConsumer == nil {
		err = errors.New("named-consumer is not attached")
		return
	}

	ts, err := me.namedConsumer.AddListener(name, receipts)
	if err != nil {
		return
	}

	tasks2 := make(chan *transport.Task, 10)
	tasks = tasks2
	go me._listener_routines_(me.listeners.New(), me.listeners.Wait(), me.events, ts, tasks2)
	return
}

func (me *defaultBridge) AddListener(rcpt <-chan *broker.Receipt) (tasks <-chan *transport.Task, err error) {
	me.consumerLock.RLock()
	defer me.consumerLock.RUnlock()

	if me.consumer == nil {
		err = errors.New("consumer is not attached")
		return
	}

	ts, err := me.consumer.AddListener(rcpt)
	if err != nil {
		return
	}

	tasks2 := make(chan *transport.Task, 10)
	tasks = tasks2
	go me._listener_routines_(me.listeners.New(), me.listeners.Wait(), me.events, ts, tasks2)
	return
}

func (me *defaultBridge) StopAllListeners() (err error) {
	me.consumerLock.RLock()
	defer me.consumerLock.RUnlock()

	if me.consumer == nil {
		return
	}

	err = me.consumer.StopAllListeners()
	if err != nil {
		return
	}

	me.listeners.Close()
	return
}

func (me *defaultBridge) Report(reports <-chan *transport.Report) (err error) {
	me.reporterLock.RLock()
	defer me.reporterLock.RUnlock()

	if me.reporter == nil {
		err = errors.New("reporter is not attached")
		return
	}

	r := make(chan *backend.Envelope, 10)
	_, err = me.reporter.Report(r)
	if err != nil {
		return
	}

	go func(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event, input <-chan *transport.Report, output chan<- *backend.Envelope) {
		defer wait.Done()
		out := func(r *transport.Report) {
			b, err := me.trans.EncodeReport(r)
			if err != nil {
				events <- common.NewEventFromError(common.InstT.BRIDGE, err)
				return
			}
			output <- &backend.Envelope{
				ID:   r,
				Body: b,
			}
		}
		for {
			select {
			case _, _ = <-quit:
				goto cleanup
			case v, ok := <-input:
				if !ok {
					goto cleanup
				}
				out(v)
			}
		}
	cleanup:
		for {
			select {
			case v, ok := <-input:
				if !ok {
					return
				}
				out(v)
			default:
				return
			}
		}
	}(me.reporters.New(), me.reporters.Wait(), me.events, reports, r)

	return
}

func (me *defaultBridge) Poll(t *transport.Task) (reports <-chan *transport.Report, err error) {
	me.storeLock.RLock()
	defer me.storeLock.RUnlock()

	if me.store == nil {
		err = errors.New("store is not attached")
		return
	}

	r, err := me.store.Poll(t)
	if err != nil {
		return
	}

	reports2 := make(chan *transport.Report, transport.Status.Count)
	reports = reports2
	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *common.Event,
		inputs <-chan []byte,
		outputs chan<- *transport.Report,
	) {
		defer wait.Done()
		out := func(b []byte) (done bool) {
			r, err := me.trans.DecodeReport(b)
			if err != nil {
				events <- common.NewEventFromError(common.InstT.STORE, err)
				return
			}
			// fix returns
			if len(r.Return()) > 0 {
				err := me.trans.Return(r)
				if err != nil {
					events <- common.NewEventFromError(common.InstT.STORE, err)
				}
			}

			outputs <- r
			done = r.Done()
			return
		}
		for {
			select {
			case _, _ = <-quit:
				goto clean
			case v, ok := <-inputs:
				if !ok {
					goto clean
				}
				if out(v) {
					goto clean
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
				out(v)
			default:
				finished = true
			}
			if finished {
				break
			}
		}
		// TODO: make sure 'Done' of backend is called
		close(outputs)
	}(me.storers.New(), me.storers.Wait(), me.storers.Events(), r, reports2)

	return
}

func (me *defaultBridge) AttachReporter(r backend.Reporter) (err error) {
	me.reporterLock.Lock()
	defer me.reporterLock.Unlock()

	if me.reporter != nil {
		err = errors.New("reporter is already attached.")
		return
	}

	if r == nil {
		err = errors.New("no reporter provided")
		return
	}

	me.reporter = r
	return
}

func (me *defaultBridge) AttachStore(s backend.Store) (err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	if me.store != nil {
		err = errors.New("store is already attached.")
		return
	}

	if s == nil {
		err = errors.New("no store provided")
		return
	}

	me.store = s
	return
}

func (me *defaultBridge) AttachProducer(p broker.Producer) (err error) {
	me.producerLock.Lock()
	defer me.producerLock.Unlock()

	if me.producer != nil {
		err = errors.New("producer is already attached.")
		return
	}

	if p == nil {
		err = errors.New("no producer provided")
		return
	}
	me.producer = p
	return
}

func (me *defaultBridge) AttachConsumer(c broker.Consumer, nc broker.NamedConsumer) (err error) {
	me.consumerLock.Lock()
	defer me.consumerLock.Unlock()

	if me.consumer != nil || me.namedConsumer != nil {
		err = errors.New("consumer is already attached.")
		return
	}

	if nc != nil {
		me.namedConsumer = nc
	} else if c != nil {
		me.consumer = c
	} else {
		err = errors.New("no consumer provided")
		return
	}

	return
}

func (me *defaultBridge) Exists(it int) (ext bool) {
	switch it {
	case common.InstT.PRODUCER:
		func() {
			me.producerLock.RLock()
			defer me.producerLock.RUnlock()

			ext = me.producer != nil
		}()
	case common.InstT.CONSUMER:
		func() {
			me.consumerLock.RLock()
			defer me.consumerLock.RUnlock()

			ext = me.consumer != nil
		}()
	case common.InstT.REPORTER:
		func() {
			me.reporterLock.RLock()
			defer me.reporterLock.RUnlock()

			ext = me.reporter != nil
		}()
	case common.InstT.STORE:
		func() {
			me.storeLock.RLock()
			defer me.storeLock.RUnlock()

			ext = me.store != nil
		}()
	case common.InstT.NAMED_CONSUMER:
		func() {
			me.consumerLock.RLock()
			defer me.consumerLock.RUnlock()

			ext = me.namedConsumer != nil
		}()
	}

	return
}

//
// routines
//

func (me *defaultBridge) _listener_routines_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *common.Event,
	input <-chan []byte,
	output chan<- *transport.Task,
) {
	defer func() {
		close(output)
		wait.Done()
	}()
	out := func(b []byte) {
		t, err := me.trans.DecodeTask(b)
		if err != nil {
			events <- common.NewEventFromError(common.InstT.BRIDGE, err)
			return
		}
		output <- t
	}
	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case v, ok := <-input:
			if !ok {
				goto cleanup
			}
			out(v)
		}
	}
cleanup:
	for {
		select {
		case v, ok := <-input:
			if !ok {
				return
			}
			out(v)
		default:
			return
		}
	}
}
