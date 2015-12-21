package dingo

import (
	"errors"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

//
// default implementation
//

type remoteBridge struct {
	producerLock  sync.RWMutex
	producer      Producer
	consumerLock  sync.RWMutex
	consumer      Consumer
	namedConsumer NamedConsumer
	reporterLock  sync.RWMutex
	reporter      Reporter
	storeLock     sync.RWMutex
	store         Store

	listeners *common.Routines
	reporters *common.Routines
	storers   *common.Routines
	doners    chan transport.Meta
	events    chan *common.Event
	eventMux  *common.Mux
	trans     *transport.Mgr
}

func newRemoteBridge(trans *transport.Mgr) (b bridge) {
	v := &remoteBridge{
		listeners: common.NewRoutines(),
		reporters: common.NewRoutines(),
		storers:   common.NewRoutines(),
		events:    make(chan *common.Event, 10),
		eventMux:  common.NewMux(),
		doners:    make(chan transport.Meta, 10),
		trans:     trans,
	}
	b = v

	v.eventMux.Handle(func(val interface{}, _ int) {
		v.events <- val.(*common.Event)
	})

	return
}

func (me *remoteBridge) Close() (err error) {
	me.listeners.Close()
	me.reporters.Close()
	me.storers.Close()

	me.eventMux.Close()
	close(me.events)
	me.events = make(chan *common.Event, 10)

	return
}

func (me *remoteBridge) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.events,
	}, nil
}

func (me *remoteBridge) SendTask(t *transport.Task) (err error) {
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

func (me *remoteBridge) AddNamedListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan *transport.Task, err error) {
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

func (me *remoteBridge) AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *transport.Task, err error) {
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

func (me *remoteBridge) StopAllListeners() (err error) {
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

func (me *remoteBridge) Report(reports <-chan *transport.Report) (err error) {
	me.reporterLock.RLock()
	defer me.reporterLock.RUnlock()

	if me.reporter == nil {
		err = errors.New("reporter is not attached")
		return
	}

	r := make(chan *ReportEnvelope, 10)
	_, err = me.reporter.Report(r)
	if err != nil {
		return
	}

	go func(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event, input <-chan *transport.Report, output chan<- *ReportEnvelope) {
		defer wait.Done()
		// raise quit signal to receiver(s).
		defer close(output)
		out := func(r *transport.Report) {
			b, err := me.trans.EncodeReport(r)
			if err != nil {
				events <- common.NewEventFromError(InstT.BRIDGE, err)
				return
			}
			output <- &ReportEnvelope{
				ID:   r,
				Body: b,
			}
		}
	finished:
		for {
			select {
			case _, _ = <-quit:
				break finished
			case v, ok := <-input:
				if !ok {
					break finished
				}
				out(v)
			}
		}

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

func (me *remoteBridge) Poll(t *transport.Task) (reports <-chan *transport.Report, err error) {
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

	reports2 := make(chan *transport.Report, Status.Count)
	reports = reports2
	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *common.Event,
		inputs <-chan []byte,
		outputs chan<- *transport.Report,
	) {
		defer wait.Done()
		defer func() {
			err = me.store.Done(t)
			if err != nil {
				events <- common.NewEventFromError(InstT.STORE, err)
			}
		}()

		var done bool
		defer func() {
			if !done {
				r, err := t.ComposeReport(Status.Fail, nil, transport.NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
				if err != nil {
					events <- common.NewEventFromError(InstT.STORE, err)
				} else {
					outputs <- r
				}
			}
			close(outputs)
		}()

		out := func(b []byte) bool {
			r, err := me.trans.DecodeReport(b)
			if err != nil {
				events <- common.NewEventFromError(InstT.STORE, err)
				return done
			}
			// fix returns
			if len(r.Return()) > 0 {
				err := me.trans.Return(r)
				if err != nil {
					events <- common.NewEventFromError(InstT.STORE, err)
				}
			}

			outputs <- r
			done = r.Done()
			return done
		}

	finished:
		for {
			select {
			case _, _ = <-quit:
				break finished
			case v, ok := <-inputs:
				_ = "breakpoint"
				if !ok {
					break finished
				}
				if out(v) {
					break finished
				}
			}
		}

	cleared:
		for {
			select {
			case v, ok := <-inputs:
				if !ok {
					break cleared
				}
				out(v)
			default:
				break cleared
			}
		}
	}(me.storers.New(), me.storers.Wait(), me.storers.Events(), r, reports2)

	return
}

func (me *remoteBridge) AttachReporter(r Reporter) (err error) {
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

func (me *remoteBridge) AttachStore(s Store) (err error) {
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

func (me *remoteBridge) AttachProducer(p Producer) (err error) {
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

func (me *remoteBridge) AttachConsumer(c Consumer, nc NamedConsumer) (err error) {
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

func (me *remoteBridge) Exists(it int) (ext bool) {
	switch it {
	case InstT.PRODUCER:
		func() {
			me.producerLock.RLock()
			defer me.producerLock.RUnlock()

			ext = me.producer != nil
		}()
	case InstT.CONSUMER:
		func() {
			me.consumerLock.RLock()
			defer me.consumerLock.RUnlock()

			ext = me.consumer != nil
		}()
	case InstT.REPORTER:
		func() {
			me.reporterLock.RLock()
			defer me.reporterLock.RUnlock()

			ext = me.reporter != nil
		}()
	case InstT.STORE:
		func() {
			me.storeLock.RLock()
			defer me.storeLock.RUnlock()

			ext = me.store != nil
		}()
	case InstT.NAMED_CONSUMER:
		func() {
			me.consumerLock.RLock()
			defer me.consumerLock.RUnlock()

			ext = me.namedConsumer != nil
		}()
	}

	return
}

func (me *remoteBridge) ReporterHook(eventID int, payload interface{}) (err error) {
	me.reporterLock.Lock()
	defer me.reporterLock.Unlock()

	if me.reporter == nil {
		return
	}

	err = me.reporter.ReporterHook(eventID, payload)
	return
}

func (me *remoteBridge) ProducerHook(eventID int, payload interface{}) (err error) {
	me.producerLock.Lock()
	defer me.producerLock.Unlock()

	if me.producer == nil {
		return
	}

	err = me.producer.ProducerHook(eventID, payload)
	return
}

//
// routines
//

func (me *remoteBridge) _listener_routines_(
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
			events <- common.NewEventFromError(InstT.BRIDGE, err)
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
