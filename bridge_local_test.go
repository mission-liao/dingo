package dingo

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type localBridgeTestSuite struct {
	bridgeTestSuite
}

func (ts *localBridgeTestSuite) SetupTest() {
	ts.bridgeTestSuite.SetupTest()
	ts.Nil(ts.bg.AttachProducer(nil))
	ts.Nil(ts.bg.AttachConsumer(nil, nil))
	ts.Nil(ts.bg.AttachReporter(nil))
	ts.Nil(ts.bg.AttachStore(nil))
}

func TestBridgeLocalSuite(t *testing.T) {
	suite.Run(t, &localBridgeTestSuite{
		bridgeTestSuite{
			name: "local",
		},
	})
}
