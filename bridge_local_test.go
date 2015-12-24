package dingo

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type localBridgeTestSuite struct {
	BridgeTestSuite
}

func (ts *localBridgeTestSuite) SetupTest() {
	ts.BridgeTestSuite.SetupTest()
	ts.Nil(ts.bg.AttachProducer(nil))
	ts.Nil(ts.bg.AttachConsumer(nil, nil))
	ts.Nil(ts.bg.AttachReporter(nil))
	ts.Nil(ts.bg.AttachStore(nil))
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
