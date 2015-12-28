package dingo

import (
	"fmt"
	"sync"
)

type localBroker struct {
	cfg *Config

	// channels
	fromUser chan []byte
	to       chan []byte

	// listener routine
	listeners *Routines
}

/*NewLocalBroker would allocate a Broker implementation based on 'channel'. Users can provide a channel and
share it between multiple Producer(s) and Consumer(s) to connect them.

This one only implements Consumer interface, not the NamedConsumer one. So
the dispatching of tasks relies on dingo.mapper
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

func (brk *localBroker) consumerRoutine(quit <-chan int, wait *sync.WaitGroup, events chan<- *Event, input <-chan []byte, output chan<- []byte, receipts <-chan *TaskReceipt) {
	defer wait.Done()
	var (
		h     *Header
		reply *TaskReceipt
		err   error
		ok    bool
		v     []byte
	)
	out := func(v []byte) (done bool) {
		if h, err = DecodeHeader(v); err != nil {
			events <- NewEventFromError(ObjT.Consumer, err)
			return
		}

		output <- v
		if reply, ok = <-receipts; !ok {
			// receipt channel is closed
			done = true
			return
		}

		if reply.ID != h.ID() {
			events <- NewEventFromError(
				ObjT.Consumer,
				fmt.Errorf("expected: %v, received: %v", h, reply),
			)
			return
		}

		return
	}

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case v, ok = <-input:
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
		default:
			break clean
		case v, ok = <-input:
			if !ok {
				break clean
			}
			if out(v) {
				break clean
			}
		}
	}
}

//
// Object interface
//

func (brk *localBroker) Expect(types int) (err error) {
	if types&^(ObjT.Producer|ObjT.Consumer) != 0 {
		err = fmt.Errorf("unsupported types: %v", types)
		return
	}

	return
}

func (brk *localBroker) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		brk.listeners.Events(),
	}, nil
}

func (brk *localBroker) Close() (err error) {
	brk.listeners.Close()
	if brk.fromUser == nil {
		// close it only when it's not provided by callers.
		close(brk.to)
	}
	brk.to = brk.fromUser
	if brk.to == nil {
		brk.to = make(chan []byte, 10)
	}
	return
}

//
// Producer
//

func (brk *localBroker) ProducerHook(eventID int, payload interface{}) (err error) {
	return
}

func (brk *localBroker) Send(id Meta, body []byte) (err error) {
	brk.to <- body
	return
}

//
// Consumer
//

func (brk *localBroker) ConsumerHook(eventID int, payload interface{}) (err error) { return }
func (brk *localBroker) AddListener(receipts <-chan *TaskReceipt) (tasks <-chan []byte, err error) {
	t := make(chan []byte, 10)
	go brk.consumerRoutine(brk.listeners.New(), brk.listeners.Wait(), brk.listeners.Events(), brk.to, t, receipts)

	tasks = t
	return
}

func (brk *localBroker) StopAllListeners() (err error) {
	brk.listeners.Close()
	return
}
