package dingo

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

type localBroker struct {
	cfg *Config

	// broker routine
	brk *common.Routines

	// channels
	fromUser chan []byte
	to       chan []byte

	// listener routine
	listeners *common.Routines
}

/*
 A Broker implementation based on 'channel'. Users can provide a channel and
 share it between multiple Producer(s) and Consumer(s) to connect them.

 This one only implements Consumer interface, not the NamedConsumer one.
*/
func NewLocalBroker(cfg *Config, to chan []byte) (v *localBroker, err error) {
	v = &localBroker{
		cfg:       cfg,
		brk:       common.NewRoutines(),
		fromUser:  to,
		to:        to,
		listeners: common.NewRoutines(),
	}

	if v.fromUser == nil {
		v.to = make(chan []byte, 10)
	}

	v.init()
	return
}

func (me *localBroker) init() (err error) {
	// broker routine
	quit := me.brk.New()
	go me._broker_routine_(quit, me.brk.Wait(), me.brk.Events())

	return
}

func (me *localBroker) _broker_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case v, ok := <-me.to:
			if !ok {
				goto clean
			}

			me.to <- v
		}
	}
clean:
}

func (me *localBroker) _consumer_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event, input <-chan []byte, output chan<- []byte, receipts <-chan *TaskReceipt) {
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
				events <- common.NewEventFromError(common.InstT.CONSUMER, err)
				break
			}

			output <- v
			reply, ok := <-receipts
			if !ok {
				goto clean
			}

			if reply.ID != h.ID() {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("expected: %v, received: %v", h, reply)),
				)
				break
			}
		}
	}
clean:
}

//
// common.Object interface
//

func (me *localBroker) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.brk.Events(),
		me.listeners.Events(),
	}, nil
}

func (me *localBroker) Close() (err error) {
	me.brk.Close()
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
