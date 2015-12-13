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

func (me *localBrokerTestSuite) SetupSuite() {
	var err error

	me.Pdc, err = NewLocalBroker(Default())
	me.Nil(err)
	me.Csm = me.Pdc.(Consumer)
	me.BrokerTestSuite.SetupSuite()
}

func (me *localBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestLocalBrokerSuite(t *testing.T) {
	suite.Run(t, &localBrokerTestSuite{})
}
