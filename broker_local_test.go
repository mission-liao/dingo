package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

//
// generic suite for Brokers
//

type localBrokerTestSuite struct {
	dingo.BrokerTestSuite
}

func TestLocalBrokerSuite(t *testing.T) {
	to := make(chan []byte, 10)
	suite.Run(t, &localBrokerTestSuite{
		dingo.BrokerTestSuite{
			Gen: func() (v interface{}, err error) {
				v, err = dingo.NewLocalBroker(dingo.DefaultConfig(), to)
				return
			},
		},
	})
}
