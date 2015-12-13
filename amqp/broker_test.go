package dgamqp

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type amqpBrokerTestSuite struct {
	dingo.BrokerTestSuite
}

func (me *amqpBrokerTestSuite) SetupSuite() {
	var err error

	me.Ncsm, err = NewBroker(DefaultAmqpConfig())
	me.Nil(err)
	me.Pdc = me.Ncsm.(dingo.Producer)
	me.BrokerTestSuite.SetupSuite()
}

func (me *amqpBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestAmqpBrokerSuite(t *testing.T) {
	suite.Run(t, &amqpBrokerTestSuite{})
}
