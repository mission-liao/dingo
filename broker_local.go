package dingo

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mission-liao/dingo/transport"
)

type localBroker struct {
	cfg *Config

	// channels
	fromUser chan []byte
	to       chan []byte

	// listener routine
	listeners *Routines
}

/*
 A Broker implementation based on 'channel'. Users can provide a channel and
 share it between multiple Producer(s) and Consumer(s) to connect them.

 This one only implements Consumer interface, not the NamedConsumer one.
*/
func NewLocalBroker(cfg *Config, to chan []byte) (v *localBroker, err error) {
	v = &localBroker{
		cfg:       cfg,
		fromUser:  to,
		to:        to,
		listeners: NewRoutines(),
	}

	if v.to == nil {
		v.to = make(chan []byte, 10)
	}

	return
}

func (me *localBroker) _consumer_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *Event, input <-chan []byte, output chan<- []byte, receipts <-chan *TaskReceipt) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case v, ok := <-input:
			if !ok {
				goto clean
			}

			h, err := transport.DecodeHeader(v)
			if err != nil {
				events <- NewEventFromError(InstT.CONSUMER, err)
				break
			}

			output <- v
			reply, ok := <-receipts
			if !ok {
				goto clean
			}

			if reply.ID != h.ID() {
				events <- NewEventFromError(
					InstT.CONSUMER,
					errors.New(fmt.Sprintf("expected: %v, received: %v", h, reply)),
				)
				break
			}
		}
	}
clean:
}

//
// Object interface
//

func (me *localBroker) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		me.listeners.Events(),
	}, nil
}

func (me *localBroker) Close() (err error) {
	me.listeners.Close()
	if me.fromUser == nil {
		// close it only when it's not provided by callers.
		close(me.to)
	}
	me.to = me.fromUser
	if me.to == nil {
		me.to = make(chan []byte, 10)
	}
	return
}

//
// Producer
//

func (me *localBroker) ProducerHook(eventID int, payload interface{}) (err error) {
	return
}

func (me *localBroker) Send(id transport.Meta, body []byte) (err error) {
	me.to <- body
	return
}

//
// Consumer
//

func (me *localBroker) AddListener(receipts <-chan *TaskReceipt) (tasks <-chan []byte, err error) {
	t := make(chan []byte, 10)
	go me._consumer_routine_(me.listeners.New(), me.listeners.Wait(), me.listeners.Events(), me.to, t, receipts)

	tasks = t
	return
}

func (me *localBroker) StopAllListeners() (err error) {
	me.listeners.Close()
	return
}
