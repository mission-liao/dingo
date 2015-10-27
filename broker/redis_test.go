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
	me._broker, err = New("redis", Default())
	me.Nil(err)
}

func (me *RedisBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestRedisBrokerSuite(t *testing.T) {
	suite.Run(t, &RedisBrokerTestSuite{})
}
