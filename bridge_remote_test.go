package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type remoteBridgeTestSuite struct {
	BridgeTestSuite

	brk Broker
	bkd Backend
}

func (me *remoteBridgeTestSuite) SetupTest() {
	var err error

	me.BridgeTestSuite.SetupTest()

	// broker
	me.brk, err = NewLocalBroker(Default())
	me.Nil(err)
	me.Nil(me.bg.AttachProducer(me.brk.(Producer)))
	me.Nil(me.bg.AttachConsumer(me.brk.(Consumer), nil))

	// backend
	me.bkd, err = NewLocalBackend(Default())
	me.Nil(err)
	me.Nil(me.bg.AttachReporter(me.bkd.(Reporter)))
	me.Nil(me.bg.AttachStore(me.bkd.(Store)))
}

func (me *remoteBridgeTestSuite) TearDownTest() {
	me.Nil(me.brk.(common.Object).Close())
	me.Nil(me.bkd.(common.Object).Close())

	me.BridgeTestSuite.TearDownTest()
}

func TestBridgeRemoteSuite(t *testing.T) {
	suite.Run(t, &remoteBridgeTestSuite{
		BridgeTestSuite: BridgeTestSuite{
			name: "",
		},
	})
}

//
// test cases
//

func (me *remoteBridgeTestSuite) TestReturnFix() {
	// register a function, returning float64
	me.Nil(me.trans.Register(
		"ReturnFix",
		func() float64 { return 0 },
		transport.Encode.Default, transport.Encode.Default,
	))

	// compose a task
	t, err := transport.ComposeTask("ReturnFix", nil, nil)
	me.Nil(err)

	// compose a corresponding report
	r, err := t.ComposeReport(transport.Status.Success, []interface{}{int(6)}, nil)
	me.Nil(err)

	// attach a reporting channel
	reports := make(chan *transport.Report, 10)
	me.Nil(me.bg.Report(reports))

	// poll the task
	outputs, err := me.bg.Poll(t)
	me.Nil(err)

	reports <- r
	out, ok := <-outputs
	me.True(ok)
	me.Len(out.Return(), 1)
	if len(out.Return()) > 0 {
		v, ok := out.Return()[0].(float64)
		me.True(ok)
		me.Equal(float64(6), v)
	}
}
