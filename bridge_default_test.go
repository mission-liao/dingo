package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
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

func (me *DefaultBridgeTestSuite) TestReturnFix() {
	// register a function, returning float64
	me.Nil(me.mash.Register("ReturnFix", func() float64 { return 0 }, transport.Encode.Default, transport.Encode.Default))
	err := me.bg.Register("ReturnFix", func() float64 { return 0 })

	// compose a task
	t, err := me.ivk.ComposeTask("ReturnFix", nil)
	me.Nil(err)

	// compose a corresponding report
	r, err := t.ComposeReport(transport.Status.Done, []interface{}{int(6)}, nil)
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
