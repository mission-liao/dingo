package dingo

import (
	"fmt"
	"sync"

	"github.com/stretchr/testify/suite"
)

/*
 All dingo.Broker provider should pass this test suite.
 Example testing code:
  type myBrokerTestSuite struct {
    dingo.BrokerTestSuite
  }
  func TestMyBrokerTestSuite(t *testing.T) {
    suite.Run(t, &myBrokerTestSuite{
      dingo.BrokerTestSuite{
        Gen: func() (interface{}, error) {
          // generate a new instance of your backend.
          // both dingo.Broker and dingo.NamedBroker are acceptable.
        },
      },
    })
  }
*/
type BrokerTestSuite struct {
	suite.Suite

	Trans         *fnMgr
	Gen           func() (interface{}, error)
	Pdc           Producer
	Csm           Consumer
	Ncsm          NamedConsumer
	ConsumerNames []string
}

func (ts *BrokerTestSuite) SetupSuite() {
}

func (ts *BrokerTestSuite) TearDownSuite() {
}

func (ts *BrokerTestSuite) SetupTest() {
	var ok bool
	v, err := ts.Gen()
	ts.Nil(err)

	// producer
	ts.Pdc = v.(Producer)

	// named consumer
	ts.Ncsm, ok = v.(NamedConsumer)
	if !ok {
		// consumer
		ts.Csm = v.(Consumer)
	}

	ts.ConsumerNames = []string{}
	ts.Trans = newFnMgr("")
}

func (ts *BrokerTestSuite) TearDownTest() {
	ts.Nil(ts.Pdc.(Object).Close())
}

func (ts *BrokerTestSuite) addListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan []byte, err error) {
	if ts.Csm != nil {
		tasks, err = ts.Csm.AddListener(receipts)
	} else if ts.Ncsm != nil {
		tasks, err = ts.Ncsm.AddListener("", receipts)
	}

	return
}

func (ts *BrokerTestSuite) stopAllListeners() (err error) {
	if ts.Csm != nil {
		err = ts.Csm.StopAllListeners()
	} else if ts.Ncsm != nil {
		err = ts.Ncsm.StopAllListeners()
	}

	return
}

//
// test cases
//

func (ts *BrokerTestSuite) TestBasic() {
	var (
		tasks <-chan []byte
	)
	ts.Nil(ts.Trans.Register("", func() {}))
	// init one listener
	receipts := make(chan *TaskReceipt, 10)
	tasks, err := ts.addListener("", receipts)
	ts.Nil(err)
	ts.NotNil(tasks)
	if tasks == nil {
		return
	}

	// compose a task
	t, err := ts.Trans.ComposeTask("", nil, []interface{}{})
	ts.Nil(err)
	ts.NotNil(t)
	if t == nil {
		return
	}

	// generate a header byte stream
	hb, err := NewHeader(t.ID(), t.Name()).Flush(0)
	ts.Nil(err)
	if err != nil {
		return
	}

	// declare this task to broker before sending
	ts.Nil(ts.Pdc.ProducerHook(ProducerEvent.DeclareTask, ""))

	// send it
	input := append(hb, []byte("test byte array")...)
	ts.Nil(ts.Pdc.Send(t, input))

	// receive it
	output := <-tasks
	ts.Equal(string(input), string(output))

	// send a receipt
	receipts <- &TaskReceipt{
		ID:     t.ID(),
		Status: ReceiptStatus.OK,
	}

	// stop all listener
	ts.Nil(ts.stopAllListeners())
}

func (ts *BrokerTestSuite) simplifiedMapper(
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
			ts.Nil(err)
			if len(name) > 0 {
				ts.Equal(name, h.Name())
			}

			fn(h)

			if len(name) > 0 && h.Name() != name {
				rcs <- &TaskReceipt{
					ID:     h.ID(),
					Status: ReceiptStatus.WorkerNotFound,
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

func (ts *BrokerTestSuite) TestNamed() {
	// make sure named consumers won't receive
	// un-registered tasks
	countOfConsumers := 5
	countOfTasks := 256

	// this task is only for NamedConsumer
	if ts.Ncsm == nil {
		return
	}

	rs := NewRoutines()
	consumers := []NamedConsumer{}

	var (
		sentLock sync.Mutex
		sent     = map[string][]string{}
		sented   sync.WaitGroup
	)

	// generate several consumers
	for i := 0; i < countOfConsumers; i++ {
		b, err := ts.Gen()
		ts.Nil(err)
		c := b.(NamedConsumer)
		rc := make(chan *TaskReceipt, 10)
		name := fmt.Sprintf("named.%d", i)
		sent[name] = []string{}
		tasks, err := c.AddListener(name, rc)
		ts.Nil(err)

		// declare this task through Producer
		ts.Nil(ts.Pdc.ProducerHook(ProducerEvent.DeclareTask, name))

		// a simplified mapper routine
		go ts.simplifiedMapper(rs.New(), rs.Wait(), tasks, rc, name,
			func(h *Header) {
				sentLock.Lock()
				defer sentLock.Unlock()

				ts.Equal(sent[name][0], h.ID())
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
		ts.ConsumerNames = append(ts.ConsumerNames, name)
	}

	// a producer sends a series of tasks
	p, err := ts.Gen()
	ts.Nil(err)
	sender := p.(Producer)
	// declare this task to broker
	sented.Add(countOfTasks)
	for i := 0; i < countOfTasks; i++ {
		// 0 ~ 1024 -> 0x0 ~ 0x400
		id, name := fmt.Sprintf("00000000-0000-0000-0000-000000000%03x", i), fmt.Sprintf("named.%d", i%countOfConsumers)
		h := NewHeader(id, name)
		b, err := h.Flush(0)
		ts.Nil(err)
		func() {
			sentLock.Lock()
			defer sentLock.Unlock()

			sent[name] = append(sent[name], id)
		}()
		err = sender.Send(h, b)
		ts.Nil(err)
		if err != nil {
			// one task is not successfully sent
			sented.Done()
		}
	}

	// wait until all tasks received
	sented.Wait()

	// make sure nothing left
	for _, v := range sent {
		ts.Len(v, 0)
	}

	for _, v := range consumers {
		ts.Nil(v.StopAllListeners())
	}

	// close all checking routines
	rs.Close()
}

func (ts *BrokerTestSuite) TestDuplicated() {
	// make sure one task won't be delievered to
	// more than once.
	countOfTasks := 256
	countOfConsumers := 5

	var (
		sentLock  sync.Mutex
		sent      = make(map[string]int)
		sented    sync.WaitGroup
		rs        = NewRoutines()
		consumers = make([]interface{}, 0, countOfConsumers)
	)

	for i := 0; i < countOfConsumers; i++ {
		var (
			err   error
			tasks <-chan []byte
			rc    = make(chan *TaskReceipt, 10)
			name  string
		)
		v, err := ts.Gen()
		ts.Nil(err)

		name = fmt.Sprintf("duplicated.%d", i)
		ts.Nil(ts.Pdc.ProducerHook(ProducerEvent.DeclareTask, name))

		if _, ok := v.(NamedConsumer); ok {
			tasks, err = v.(NamedConsumer).AddListener(name, rc)
			ts.ConsumerNames = append(ts.ConsumerNames, name)
		} else {
			tasks, err = v.(Consumer).AddListener(rc)
			name = ""
		}
		ts.Nil(err)

		go ts.simplifiedMapper(rs.New(), rs.Wait(), tasks, rc, name,
			func(h *Header) {
				sentLock.Lock()
				defer sentLock.Unlock()

				_, ok := sent[h.ID()]
				ts.True(ok)
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
	p, err := ts.Gen()
	ts.Nil(err)
	sender := p.(Producer)
	sented.Add(countOfTasks)
	for i := 0; i < countOfTasks; i++ {
		// 0 ~ 1024 -> 0x0 ~ 0x400
		id, name := fmt.Sprintf("00000000-0000-0000-0000-000000000%03x", i), fmt.Sprintf("duplicated.%d", i%countOfConsumers)
		h := NewHeader(id, name)
		b, err := h.Flush(0)
		ts.Nil(err)
		func() {
			sentLock.Lock()
			defer sentLock.Unlock()

			// init a slot with zero
			sent[id] = 0
		}()
		err = sender.Send(h, b)
		ts.Nil(err)
	}

	// wait until all task received
	sented.Wait()

	// close all routines
	rs.Close()

	// verify that all slot in 'sent' should be 1
	for _, v := range sent {
		ts.Equal(1, v)
	}
}

func (ts *BrokerTestSuite) TestExpect() {
	ts.NotNil(ts.Pdc.(Object).Expect(ObjT.Reporter))
	ts.NotNil(ts.Pdc.(Object).Expect(ObjT.Store))
	ts.NotNil(ts.Pdc.(Object).Expect(ObjT.All))
}
