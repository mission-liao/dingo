package dingo

import (
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BrokerTestSuite struct {
	suite.Suite

	Gen  func() (interface{}, error)
	Pdc  Producer
	Csm  Consumer
	Ncsm NamedConsumer
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
}

func (me *BrokerTestSuite) TearDownTest() {
	me.Nil(me.Pdc.(common.Object).Close())
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
	// init one listener
	receipts := make(chan *TaskReceipt, 10)
	tasks, err := me.AddListener("", receipts)
	me.Nil(err)
	me.NotNil(tasks)
	if tasks == nil {
		return
	}

	// compose a task
	t, err := transport.ComposeTask("", nil, []interface{}{})
	me.Nil(err)
	me.NotNil(t)
	if t == nil {
		return
	}

	// generate a header byte stream
	hb, err := transport.NewHeader(t.ID(), t.Name()).Flush(0)
	me.Nil(err)
	if err != nil {
		return
	}

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
