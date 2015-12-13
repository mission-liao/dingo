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
	to  chan []byte

	// listener routine
	listeners *common.Routines
}

// factory
func NewLocalBroker(cfg *Config) (v *localBroker, err error) {
	v = &localBroker{
		cfg:       cfg,
		brk:       common.NewRoutines(),
		to:        make(chan []byte, 10),
		listeners: common.NewRoutines(),
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
	close(me.to)
	me.to = make(chan []byte, 10)
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
