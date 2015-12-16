package dingo

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

//
// generic suite for Brokers
//

type localBrokerTestSuite struct {
	BrokerTestSuite
}

func TestLocalBrokerSuite(t *testing.T) {
	suite.Run(t, &localBrokerTestSuite{
		BrokerTestSuite{
			Gen: func() (v interface{}, err error) {
				v, err = NewLocalBroker(Default())
				return
			},
		},
	})
}
