package broker

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RedisBrokerTestSuite struct {
	BrokerTestSuite
}

func (me *RedisBrokerTestSuite) SetupSuite() {
	var err error

	me.BrokerTestSuite.SetupSuite()
	obj, err := NewNamed("redis", Default())
	me.Nil(err)
	me._producer = obj.(Producer)
	me._namedConsumer = obj.(NamedConsumer)
}

func (me *RedisBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestRedisBrokerSuite(t *testing.T) {
	suite.Run(t, &RedisBrokerTestSuite{})
}
