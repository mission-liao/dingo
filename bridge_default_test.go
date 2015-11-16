package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/stretchr/testify/suite"
)

type DefaultBridgeTestSuite struct {
	BridgeTestSuite

	brk broker.Broker
	bkd backend.Backend
}

func (me *DefaultBridgeTestSuite) SetupTest() {
	var err error

	me.BridgeTestSuite.SetupTest()

	// broker
	me.brk, err = broker.New("local", broker.Default())
	me.Nil(err)
	me.Nil(me.bg.AttachProducer(me.brk.(broker.Producer)))
	me.Nil(me.bg.AttachConsumer(me.brk.(broker.Consumer)))

	// backend
	me.bkd, err = backend.New("local", backend.Default())
	me.Nil(err)
	me.Nil(me.bg.AttachReporter(me.bkd.(backend.Reporter)))
	me.Nil(me.bg.AttachStore(me.bkd.(backend.Store)))
}

func (me *DefaultBridgeTestSuite) TearDownTest() {
	me.Nil(me.brk.(common.Object).Close())
	me.Nil(me.bkd.(common.Object).Close())

	me.BridgeTestSuite.TearDownTest()
}

func TestBridgeDefaultSuite(t *testing.T) {
	suite.Run(t, &DefaultBridgeTestSuite{
		BridgeTestSuite: BridgeTestSuite{
			name: "",
		},
	})
}

//
// test cases
//
