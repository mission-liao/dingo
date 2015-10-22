package broker

import (
	"testing"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/suite"
)

type AmqpBrokerTestSuite struct {
	suite.Suite

	_broker  Broker
	_invoker meta.Invoker
}

func (me *AmqpBrokerTestSuite) SetupSuite() {
	var err error

	me._broker, err = New("amqp", Default())
	me.Nil(err)
}

func (me *AmqpBrokerTestSuite) TearDownSuite() {
	me.Nil(me._broker.(common.Server).Close())
}

func TestAmqpBrokerSuite(t *testing.T) {
	suite.Run(t, &AmqpBrokerTestSuite{
		_invoker: meta.NewDefaultInvoker(),
	})
}

//
// test cases
//

func (me *AmqpBrokerTestSuite) TestBasic() {
	// init one listener
	receipts := make(chan Receipt, 10)
	tasks, _, err := me._broker.AddListener(receipts)
	me.Nil(err)

	// compose a task
	t, err := me._invoker.ComposeTask("test")
	me.Nil(err)
	me.NotNil(t)
	if t == nil {
		return
	}

	// send it
	me.Nil(me._broker.Send(t))

	// receive it
	t2 := <-tasks
	me.Equal(t2, t)

	// send a receipt
	receipts <- Receipt{
		Status: Status.OK,
	}

	// stop all listener
	me.Nil(me._broker.Stop())
}
