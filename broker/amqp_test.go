package broker

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AmqpBrokerTestSuite struct {
	BrokerTestSuite
}

func (me *AmqpBrokerTestSuite) SetupSuite() {
	var err error

	me.BrokerTestSuite.SetupSuite()
	obj, err := NewNamed("amqp", Default())
	me.Nil(err)
	me._namedConsumer = obj.(NamedConsumer)
	me._producer = obj.(Producer)
}

func (me *AmqpBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestAmqpBrokerSuite(t *testing.T) {
	suite.Run(t, &AmqpBrokerTestSuite{})
}
