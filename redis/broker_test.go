package dgredis

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type RedisBrokerTestSuite struct {
	dingo.BrokerTestSuite
}

func TestRedisBrokerSuite(t *testing.T) {
	suite.Run(t, &RedisBrokerTestSuite{
		dingo.BrokerTestSuite{
			Gen: func() (b interface{}, err error) {
				b, err = NewBroker(DefaultRedisConfig())
				return
			},
		},
	})
}
