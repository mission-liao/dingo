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

func (bdg *remoteBridge) Close() (err error) {
	bdg.listeners.Close()
	bdg.reporters.Close()
	bdg.storers.Close()

	close(bdg.events)
	bdg.events = make(chan *Event, 10)

	return
}

func (bdg *remoteBridge) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		bdg.events,
	}, nil
}

func (bdg *remoteBridge) SendTask(t *Task) (err error) {
	bdg.producerLock.RLock()
	defer bdg.producerLock.RUnlock()

	if bdg.producer == nil {
		err = errors.New("producer is not attached")
		return
	}

	b, err := bdg.trans.EncodeTask(t)
	if err != nil {
		return
	}

	err = bdg.producer.Send(t, b)
	return
}

func (bdg *remoteBridge) AddNamedListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	bdg.consumerLock.RLock()
	defer bdg.consumerLock.RUnlock()

	if bdg.namedConsumer == nil {
		err = errors.New("named-consumer is not attached")
		return
	}

	ts, err := bdg.namedConsumer.AddListener(name, receipts)
	if err != nil {
		return
	}

	tasks2 := make(chan *Task, 10)
	tasks = tasks2
	go bdg.listenerRoutines(bdg.listeners.New(), bdg.listeners.Wait(), bdg.events, ts, tasks2)
	return
}

func (bdg *remoteBridge) AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	bdg.consumerLock.RLock()
	defer bdg.consumerLock.RUnlock()

	if bdg.consumer == nil {
		err = errors.New("consumer is not attached")
		return
	}

	ts, err := bdg.consumer.AddListener(rcpt)
	if err != nil {
		return
	}

	tasks2 := make(chan *Task, 10)
	tasks = tasks2
	go bdg.listenerRoutines(bdg.listeners.New(), bdg.listeners.Wait(), bdg.events, ts, tasks2)
	return
}

func (bdg *remoteBridge) StopAllListeners() (err error) {
	bdg.consumerLock.RLock()
	defer bdg.consumerLock.RUnlock()

	if bdg.consumer != nil {
		err = bdg.consumer.StopAllListeners()
		if err != nil {
			return
		}
	} else if bdg.namedConsumer != nil {
		err = bdg.namedConsumer.StopAllListeners()
		if err != nil {
			return
		}
	}

	bdg.listeners.Close()
	return
}

func (bdg *remoteBridge) Report(reports <-chan *Report) (err error) {
	bdg.reporterLock.RLock()
	defer bdg.reporterLock.RUnlock()

	if bdg.reporter == nil {
		err = errors.New("reporter is not attached")
		return
	}

	r := make(chan *ReportEnvelope, 10)
	_, err = bdg.reporter.Report(r)
	if err != nil {
		return
	}

	go func(quit <-chan int, wait *sync.WaitGroup, events chan<- *Event, input <-chan *Report, output chan<- *ReportEnvelope) {
		defer wait.Done()
		// raise quit signal to receiver(s).
		defer close(output)
		out := func(r *Report) {
			b, err := bdg.trans.EncodeReport(r)
			if err != nil {
				events <- NewEventFromError(ObjT.Bridge, err)
				return
			}
			output <- &ReportEnvelope{
				ID:   r,
				Body: b,
			}
		}
		for {
			select {
			case _, _ = <-quit:
				goto clean
			case v, ok := <-input:
				if !ok {
					goto clean
				}
				out(v)
			}
		}
	clean:
		for {
			select {
			case v, ok := <-input:
				if !ok {
					break clean
				}
				out(v)
			default:
				break clean
			}
		}
	}(bdg.reporters.New(), bdg.reporters.Wait(), bdg.events, reports, r)

	return
}

func (bdg *remoteBridge) Poll(t *Task) (reports <-chan *Report, err error) {
	bdg.storeLock.RLock()
	defer bdg.storeLock.RUnlock()

	if bdg.store == nil {
		err = errors.New("store is not attached")
		return
	}

	r, err := bdg.store.Poll(t)
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
			err = bdg.store.Done(t)
			if err != nil {
				events <- NewEventFromError(ObjT.Store, err)
			}
		}()

		var done bool
		defer func() {
			if !done {
				r, err := t.composeReport(Status.Fail, nil, NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
				if err != nil {
					events <- NewEventFromError(ObjT.Store, err)
				} else {
					outputs <- r
				}
			}
			close(outputs)
		}()

		out := func(b []byte) bool {
			r, err := bdg.trans.DecodeReport(b)
			if err != nil {
				events <- NewEventFromError(ObjT.Store, err)
				return done
			}
			// fix returns
			if len(r.Return()) > 0 {
				err := bdg.trans.Return(r)
				if err != nil {
					events <- NewEventFromError(ObjT.Store, err)
				}
			}

			outputs <- r
			done = r.Done()
			return done
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
		for {
			select {
			case v, ok := <-inputs:
				if !ok {
					break clean
				}
				out(v)
			default:
				break clean
			}
		}
	}(bdg.storers.New(), bdg.storers.Wait(), bdg.storers.Events(), r, reports2)

	return
}

func (bdg *remoteBridge) AttachReporter(r Reporter) (err error) {
	bdg.reporterLock.Lock()
	defer bdg.reporterLock.Unlock()

	if bdg.reporter != nil {
		err = errors.New("reporter is already attached")
		return
	}

	if r == nil {
		err = errors.New("no reporter provided")
		return
	}

	bdg.reporter = r
	return
}

func (bdg *remoteBridge) AttachStore(s Store) (err error) {
	bdg.storeLock.Lock()
	defer bdg.storeLock.Unlock()

	if bdg.store != nil {
		err = errors.New("store is already attached")
		return
	}

	if s == nil {
		err = errors.New("no store provided")
		return
	}

	bdg.store = s
	return
}

func (bdg *remoteBridge) AttachProducer(p Producer) (err error) {
	bdg.producerLock.Lock()
	defer bdg.producerLock.Unlock()

	if bdg.producer != nil {
		err = errors.New("producer is already attached")
		return
	}

	if p == nil {
		err = errors.New("no producer provided")
		return
	}
	bdg.producer = p
	return
}

func (bdg *remoteBridge) AttachConsumer(c Consumer, nc NamedConsumer) (err error) {
	bdg.consumerLock.Lock()
	defer bdg.consumerLock.Unlock()

	if bdg.consumer != nil || bdg.namedConsumer != nil {
		err = errors.New("consumer is already attached")
		return
	}

	if nc != nil {
		bdg.namedConsumer = nc
	} else if c != nil {
		bdg.consumer = c
	} else {
		err = errors.New("no consumer provided")
		return
	}

	return
}

func (bdg *remoteBridge) Exists(it int) (ext bool) {
	switch it {
	case ObjT.Producer:
		func() {
			bdg.producerLock.RLock()
			defer bdg.producerLock.RUnlock()

			ext = bdg.producer != nil
		}()
	case ObjT.Consumer:
		func() {
			bdg.consumerLock.RLock()
			defer bdg.consumerLock.RUnlock()

			ext = bdg.consumer != nil
		}()
	case ObjT.Reporter:
		func() {
			bdg.reporterLock.RLock()
			defer bdg.reporterLock.RUnlock()

			ext = bdg.reporter != nil
		}()
	case ObjT.Store:
		func() {
			bdg.storeLock.RLock()
			defer bdg.storeLock.RUnlock()

			ext = bdg.store != nil
		}()
	case ObjT.NamedConsumer:
		func() {
			bdg.consumerLock.RLock()
			defer bdg.consumerLock.RUnlock()

			ext = bdg.namedConsumer != nil
		}()
	}

	return
}

func (bdg *remoteBridge) ReporterHook(eventID int, payload interface{}) (err error) {
	bdg.reporterLock.Lock()
	defer bdg.reporterLock.Unlock()

	if bdg.reporter == nil {
		return
	}

	err = bdg.reporter.ReporterHook(eventID, payload)
	return
}

func (bdg *remoteBridge) StoreHook(eventID int, payload interface{}) (err error) {
	bdg.storeLock.Lock()
	defer bdg.storeLock.Unlock()

	if bdg.store == nil {
		return
	}

	err = bdg.store.StoreHook(eventID, payload)
	return
}

func (bdg *remoteBridge) ProducerHook(eventID int, payload interface{}) (err error) {
	bdg.producerLock.Lock()
	defer bdg.producerLock.Unlock()

	if bdg.producer == nil {
		return
	}

	err = bdg.producer.ProducerHook(eventID, payload)
	return
}

func (bdg *remoteBridge) ConsumerHook(eventID int, payload interface{}) (err error) {
	bdg.consumerLock.Lock()
	defer bdg.consumerLock.Unlock()

	if bdg.consumer != nil {
		err = bdg.consumer.ConsumerHook(eventID, payload)
		if err != nil {
			return
		}
	} else if bdg.namedConsumer != nil {
		err = bdg.namedConsumer.ConsumerHook(eventID, payload)
		if err != nil {
			return
		}
	}

	return
}

//
// routines
//

func (bdg *remoteBridge) listenerRoutines(
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
		t, err := bdg.trans.DecodeTask(b)
		if err != nil {
			events <- NewEventFromError(ObjT.Bridge, err)
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
