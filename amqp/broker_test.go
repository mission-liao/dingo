package dgamqp

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type amqpBrokerTestSuite struct {
	dingo.BrokerTestSuite
}

func TestAmqpBrokerSuite(t *testing.T) {
	suite.Run(t, &amqpBrokerTestSuite{
		dingo.BrokerTestSuite{
			Gen: func() (b interface{}, err error) {
				b, err = NewBroker(DefaultAmqpConfig())
				return
			},
		},
	})
}
