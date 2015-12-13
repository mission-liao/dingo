package dgredis

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type RedisBrokerTestSuite struct {
	dingo.BrokerTestSuite
}

func (me *RedisBrokerTestSuite) SetupSuite() {
	var err error

	me.Pdc, err = NewBroker(DefaultRedisConfig())
	me.Nil(err)
	me.Ncsm = me.Pdc.(dingo.NamedConsumer)
	me.BrokerTestSuite.SetupSuite()
}

func (me *RedisBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestRedisBrokerSuite(t *testing.T) {
	suite.Run(t, &RedisBrokerTestSuite{})
}
