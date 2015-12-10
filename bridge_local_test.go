package dingo

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type localBridgeTestSuite struct {
	BridgeTestSuite
}

func (me *localBridgeTestSuite) SetupTest() {
	me.BridgeTestSuite.SetupTest()
	me.Nil(me.bg.AttachProducer(nil))
	me.Nil(me.bg.AttachConsumer(nil, nil))
	me.Nil(me.bg.AttachReporter(nil))
	me.Nil(me.bg.AttachStore(nil))
}

func TestBridgeLocalSuite(t *testing.T) {
	suite.Run(t, &localBridgeTestSuite{
		BridgeTestSuite{
			name: "local",
		},
	})
}

//
// test cases
//
