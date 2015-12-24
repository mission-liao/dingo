package dingo

import (
	"errors"
	"sync"
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

	listeners *Routines
	reporters *Routines
	storers   *Routines
	doners    chan Meta
	events    chan *Event
	trans     *fnMgr
}

func newRemoteBridge(trans *fnMgr) (b bridge) {
	v := &remoteBridge{
		listeners: NewRoutines(),
		reporters: NewRoutines(),
		storers:   NewRoutines(),
		events:    make(chan *Event, 10),
		doners:    make(chan Meta, 10),
		trans:     trans,
	}
	b = v

	return
}

func (me *remoteBridge) Close() (err error) {
	me.listeners.Close()
	me.reporters.Close()
	me.storers.Close()

	close(me.events)
	me.events = make(chan *Event, 10)

	return
}

func (me *remoteBridge) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		me.events,
	}, nil
}

func (me *remoteBridge) SendTask(t *Task) (err error) {
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

func (me *remoteBridge) AddNamedListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
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

	tasks2 := make(chan *Task, 10)
	tasks = tasks2
	go me._listener_routines_(me.listeners.New(), me.listeners.Wait(), me.events, ts, tasks2)
	return
}

func (me *remoteBridge) AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
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

	tasks2 := make(chan *Task, 10)
	tasks = tasks2
	go me._listener_routines_(me.listeners.New(), me.listeners.Wait(), me.events, ts, tasks2)
	return
}

func (me *remoteBridge) StopAllListeners() (err error) {
	me.consumerLock.RLock()
	defer me.consumerLock.RUnlock()

	if me.consumer != nil {
		err = me.consumer.StopAllListeners()
		if err != nil {
			return
		}
	} else if me.namedConsumer != nil {
		err = me.namedConsumer.StopAllListeners()
		if err != nil {
			return
		}
	}

	me.listeners.Close()
	return
}

func (me *remoteBridge) Report(reports <-chan *Report) (err error) {
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

	go func(quit <-chan int, wait *sync.WaitGroup, events chan<- *Event, input <-chan *Report, output chan<- *ReportEnvelope) {
		defer wait.Done()
		// raise quit signal to receiver(s).
		defer close(output)
		out := func(r *Report) {
			b, err := me.trans.EncodeReport(r)
			if err != nil {
				events <- NewEventFromError(ObjT.BRIDGE, err)
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

func (me *remoteBridge) Poll(t *Task) (reports <-chan *Report, err error) {
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

	reports2 := make(chan *Report, Status.Count)
	reports = reports2
	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *Event,
		inputs <-chan []byte,
		outputs chan<- *Report,
	) {
		defer wait.Done()
		defer func() {
			err = me.store.Done(t)
			if err != nil {
				events <- NewEventFromError(ObjT.STORE, err)
			}
		}()

		var done bool
		defer func() {
			if !done {
				r, err := t.composeReport(Status.Fail, nil, NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
				if err != nil {
					events <- NewEventFromError(ObjT.STORE, err)
				} else {
					outputs <- r
				}
			}
			close(outputs)
		}()

		out := func(b []byte) bool {
			r, err := me.trans.DecodeReport(b)
			if err != nil {
				events <- NewEventFromError(ObjT.STORE, err)
				return done
			}
			// fix returns
			if len(r.Return()) > 0 {
				err := me.trans.Return(r)
				if err != nil {
					events <- NewEventFromError(ObjT.STORE, err)
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
	case ObjT.PRODUCER:
		func() {
			me.producerLock.RLock()
			defer me.producerLock.RUnlock()

			ext = me.producer != nil
		}()
	case ObjT.CONSUMER:
		func() {
			me.consumerLock.RLock()
			defer me.consumerLock.RUnlock()

			ext = me.consumer != nil
		}()
	case ObjT.REPORTER:
		func() {
			me.reporterLock.RLock()
			defer me.reporterLock.RUnlock()

			ext = me.reporter != nil
		}()
	case ObjT.STORE:
		func() {
			me.storeLock.RLock()
			defer me.storeLock.RUnlock()

			ext = me.store != nil
		}()
	case ObjT.NAMED_CONSUMER:
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

func (me *remoteBridge) StoreHook(eventID int, payload interface{}) (err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	if me.store == nil {
		return
	}

	err = me.store.StoreHook(eventID, payload)
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

func (me *remoteBridge) ConsumerHook(eventID int, payload interface{}) (err error) {
	me.consumerLock.Lock()
	defer me.consumerLock.Unlock()

	if me.consumer != nil {
		err = me.consumer.ConsumerHook(eventID, payload)
		if err != nil {
			return
		}
	} else if me.namedConsumer != nil {
		err = me.namedConsumer.ConsumerHook(eventID, payload)
		if err != nil {
			return
		}
	}

	return
}

//
// routines
//

func (me *remoteBridge) _listener_routines_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *Event,
	input <-chan []byte,
	output chan<- *Task,
) {
	defer func() {
		close(output)
		wait.Done()
	}()
	out := func(b []byte) {
		t, err := me.trans.DecodeTask(b)
		if err != nil {
			events <- NewEventFromError(ObjT.BRIDGE, err)
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
