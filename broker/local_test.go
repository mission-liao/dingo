package broker

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

//
// generic suite for Brokers
//

type LocalBrokerTestSuite struct {
	BrokerTestSuite
}

func (me *LocalBrokerTestSuite) SetupSuite() {
	var err error

	me.BrokerTestSuite.SetupSuite()
	me._broker, err = New("local", Default())
	me.Nil(err)
}

func (me *LocalBrokerTestSuite) TearDownSuite() {
	me.BrokerTestSuite.TearDownSuite()
}

func TestLocalBrokerSuite(t *testing.T) {
	suite.Run(t, &LocalBrokerTestSuite{})
}
