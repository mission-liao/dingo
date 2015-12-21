package dingo

import (
	"fmt"
	"sync"

	"github.com/stretchr/testify/suite"
)

type BrokerTestSuite struct {
	suite.Suite

	Trans         *Mgr
	Gen           func() (interface{}, error)
	Pdc           Producer
	Csm           Consumer
	Ncsm          NamedConsumer
	ConsumerNames []string
}

func (me *BrokerTestSuite) SetupTest() {
	var ok bool
	v, err := me.Gen()
	me.Nil(err)

	// producer
	me.Pdc = v.(Producer)

	// named consumer
	me.Ncsm, ok = v.(NamedConsumer)
	if !ok {
		// consumer
		me.Csm = v.(Consumer)
	}

	me.ConsumerNames = []string{}
	me.Trans = NewMgr()
}

func (me *BrokerTestSuite) TearDownTest() {
	me.Nil(me.Pdc.(Object).Close())
}

func (me *BrokerTestSuite) AddListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan []byte, err error) {
	if me.Csm != nil {
		tasks, err = me.Csm.AddListener(receipts)
	} else if me.Ncsm != nil {
		tasks, err = me.Ncsm.AddListener("", receipts)
	}

	return
}

func (me *BrokerTestSuite) StopAllListeners() (err error) {
	if me.Csm != nil {
		err = me.Csm.StopAllListeners()
	} else if me.Ncsm != nil {
		err = me.Ncsm.StopAllListeners()
	}

	return
}

//
// test cases
//

func (me *BrokerTestSuite) TestBasic() {
	var (
		tasks <-chan []byte
	)
	me.Nil(me.Trans.Register("", func() {}, Encode.Default, Encode.Default, ID.Default))
	// init one listener
	receipts := make(chan *TaskReceipt, 10)
	tasks, err := me.AddListener("", receipts)
	me.Nil(err)
	me.NotNil(tasks)
	if tasks == nil {
		return
	}

	// compose a task
	t, err := me.Trans.ComposeTask("", nil, []interface{}{})
	me.Nil(err)
	me.NotNil(t)
	if t == nil {
		return
	}

	// generate a header byte stream
	hb, err := NewHeader(t.ID(), t.Name()).Flush(0)
	me.Nil(err)
	if err != nil {
		return
	}

	// declare this task to broker before sending
	me.Nil(me.Pdc.ProducerHook(ProducerEvent.DeclareTask, ""))

	// send it
	input := append(hb, []byte("test byte array")...)
	me.Nil(me.Pdc.Send(t, input))

	// receive it
	output := <-tasks
	me.Equal(string(input), string(output))

	// send a receipt
	receipts <- &TaskReceipt{
		ID:     t.ID(),
		Status: ReceiptStatus.OK,
	}

	// stop all listener
	me.Nil(me.StopAllListeners())
}

func (me *BrokerTestSuite) _simplified_mapper_(
	quit <-chan int,
	wait *sync.WaitGroup,
	tasks <-chan []byte,
	rcs chan<- *TaskReceipt,
	name string,
	fn func(h *Header),
) {
	defer wait.Done()
	defer close(rcs)
done:
	for {
		select {
		case _, _ = <-quit:
			break done
		case t, ok := <-tasks:
			if !ok {
				break done
			}
			h, err := DecodeHeader(t)
			me.Nil(err)
			if len(name) > 0 {
				me.Equal(name, h.Name())
			}

			fn(h)

			if len(name) > 0 && h.Name() != name {
				rcs <- &TaskReceipt{
					ID:     h.ID(),
					Status: ReceiptStatus.WORKER_NOT_FOUND,
				}
			} else {
				rcs <- &TaskReceipt{
					ID:     h.ID(),
					Status: ReceiptStatus.OK,
				}
			}
		}
	}

}

func (me *BrokerTestSuite) TestNamed() {
	// make sure named consumers won't receive
	// un-registered tasks
	countOfConsumers := 5
	countOfTasks := 256

	// this task is only for NamedConsumer
	if me.Ncsm == nil {
		return
	}

	rs := NewRoutines()
	consumers := []NamedConsumer{}

	var (
		sentLock sync.Mutex
		sent     map[string][]string = map[string][]string{}
		sented   sync.WaitGroup
	)

	// generate several consumers
	for i := 0; i < countOfConsumers; i++ {
		b, err := me.Gen()
		me.Nil(err)
		c := b.(NamedConsumer)
		rc := make(chan *TaskReceipt, 10)
		name := fmt.Sprintf("named.%d", i)
		sent[name] = []string{}
		tasks, err := c.AddListener(name, rc)
		me.Nil(err)

		// declare this task through Producer
		me.Nil(me.Pdc.ProducerHook(ProducerEvent.DeclareTask, name))

		// a simplified mapper routine
		go me._simplified_mapper_(rs.New(), rs.Wait(), tasks, rc, name,
			func(h *Header) {
				sentLock.Lock()
				defer sentLock.Unlock()

				me.Equal(sent[name][0], h.ID())
				// remove this 'sent' record
				for k, v := range sent[name] {
					if v == h.ID() {
						sent[name] = append(sent[name][:k], sent[name][k+1:]...)
						break
					}
				}

				// received one task
				sented.Done()
			},
		)
		consumers = append(consumers, c)
		me.ConsumerNames = append(me.ConsumerNames, name)
	}

	// a producer sends a series of tasks
	p, err := me.Gen()
	me.Nil(err)
	sender := p.(Producer)
	// declare this task to broker
	sented.Add(countOfTasks)
	for i := 0; i < countOfTasks; i++ {
		// 0 ~ 1024 -> 0x0 ~ 0x400
		id, name := fmt.Sprintf("00000000-0000-0000-0000-000000000%03x", i), fmt.Sprintf("named.%d", i%countOfConsumers)
		h := NewHeader(id, name)
		b, err := h.Flush(0)
		me.Nil(err)
		func() {
			sentLock.Lock()
			defer sentLock.Unlock()

			sent[name] = append(sent[name], id)
		}()
		err = sender.Send(h, b)
		me.Nil(err)
		if err != nil {
			// one task is not successfully sent
			sented.Done()
		}
	}

	// wait until all tasks received
	sented.Wait()

	// make sure nothing left
	for _, v := range sent {
		me.Len(v, 0)
	}

	for _, v := range consumers {
		me.Nil(v.StopAllListeners())
	}

	// close all checking routines
	rs.Close()
}

func (me *BrokerTestSuite) TestDuplicated() {
	// make sure one task won't be delievered to
	// more than once.
	countOfTasks := 256
	countOfConsumers := 5

	var (
		sentLock  sync.Mutex
		sent      map[string]int = make(map[string]int)
		sented    sync.WaitGroup
		rs        *Routines     = NewRoutines()
		consumers []interface{} = make([]interface{}, 0, countOfConsumers)
	)

	for i := 0; i < countOfConsumers; i++ {
		var (
			err   error
			tasks <-chan []byte
			rc    chan *TaskReceipt = make(chan *TaskReceipt, 10)
			name  string
		)
		v, err := me.Gen()
		me.Nil(err)

		name = fmt.Sprintf("duplicated.%d", i)
		me.Nil(me.Pdc.ProducerHook(ProducerEvent.DeclareTask, name))

		if _, ok := v.(NamedConsumer); ok {
			tasks, err = v.(NamedConsumer).AddListener(name, rc)
			me.ConsumerNames = append(me.ConsumerNames, name)
		} else {
			tasks, err = v.(Consumer).AddListener(rc)
			name = ""
		}
		me.Nil(err)

		go me._simplified_mapper_(rs.New(), rs.Wait(), tasks, rc, name,
			func(h *Header) {
				sentLock.Lock()
				defer sentLock.Unlock()

				_, ok := sent[h.ID()]
				me.True(ok)
				if ok {
					sent[h.ID()]++
					sented.Done()
				} else {
					sent[h.ID()] = 1
				}
			},
		)

		consumers = append(consumers, v)
	}

	// a producer sends a series of tasks
	p, err := me.Gen()
	me.Nil(err)
	sender := p.(Producer)
	sented.Add(countOfTasks)
	for i := 0; i < countOfTasks; i++ {
		// 0 ~ 1024 -> 0x0 ~ 0x400
		id, name := fmt.Sprintf("00000000-0000-0000-0000-000000000%03x", i), fmt.Sprintf("duplicated.%d", i%countOfConsumers)
		h := NewHeader(id, name)
		b, err := h.Flush(0)
		me.Nil(err)
		func() {
			sentLock.Lock()
			defer sentLock.Unlock()

			// init a slot with zero
			sent[id] = 0
		}()
		err = sender.Send(h, b)
		me.Nil(err)
	}

	// wait until all task received
	sented.Wait()

	// close all routines
	rs.Close()

	// verify that all slot in 'sent' should be 1
	for _, v := range sent {
		me.Equal(1, v)
	}
}
