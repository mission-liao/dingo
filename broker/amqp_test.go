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
	me._broker, err = New("amqp", Default())
	me.Nil(err)
}

func (me *AmqpBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestAmqpBrokerSuite(t *testing.T) {
	suite.Run(t, &AmqpBrokerTestSuite{})
}
