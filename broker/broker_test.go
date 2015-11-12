package broker

import (
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BrokerTestSuite struct {
	suite.Suite

	_broker  Broker
	_invoker transport.Invoker
}

func (me *BrokerTestSuite) SetupSuite() {
	me._invoker = transport.NewDefaultInvoker()
	me.NotNil(me._invoker)
}

func (me *BrokerTestSuite) TearDownSuite() {
	me.Nil(me._broker.(common.Object).Close())
}

//
// test cases
//

func (me *BrokerTestSuite) TestBasic() {
	// init one listener
	receipts := make(chan *Receipt, 10)
	tasks, err := me._broker.AddListener(receipts)
	me.Nil(err)

	// compose a task
	t, err := me._invoker.ComposeTask("basic", []interface{}{})
	me.Nil(err)
	me.NotNil(t)
	if t == nil {
		return
	}

	// send it
	input := append(
		transport.EncodeHeader(t.ID(), transport.Encode.Default),
		[]byte("test byte array")...,
	)
	me.Nil(me._broker.Send(t, input))

	// receive it
	output := <-tasks
	me.Equal(string(input), string(output))

	// send a receipt
	receipts <- &Receipt{
		ID:     t.ID(),
		Status: Status.OK,
	}

	// stop all listener
	me.Nil(me._broker.StopAllListeners())
}
