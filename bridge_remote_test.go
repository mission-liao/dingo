package dingo

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type remoteBridgeTestSuite struct {
	BridgeTestSuite

	brk Broker
	bkd Backend
}

func (ts *remoteBridgeTestSuite) SetupTest() {
	var err error

	ts.BridgeTestSuite.SetupTest()

	// broker
	ts.brk, err = NewLocalBroker(DefaultConfig(), nil)
	ts.Nil(err)
	ts.Nil(ts.bg.AttachProducer(ts.brk.(Producer)))
	ts.Nil(ts.bg.AttachConsumer(ts.brk.(Consumer), nil))
	ts.Nil(ts.brk.(Object).Expect(ObjT.Producer | ObjT.Consumer))

	// backend
	ts.bkd, err = NewLocalBackend(DefaultConfig(), nil)
	ts.Nil(err)
	ts.Nil(ts.bg.AttachReporter(ts.bkd.(Reporter)))
	ts.Nil(ts.bg.AttachStore(ts.bkd.(Store)))
	ts.Nil(ts.bkd.(Object).Expect(ObjT.Reporter | ObjT.Store))
}

func (ts *remoteBridgeTestSuite) TearDownTest() {
	ts.Nil(ts.brk.(Object).Close())
	ts.Nil(ts.bkd.(Object).Close())

	ts.BridgeTestSuite.TearDownTest()
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

func (ts *remoteBridgeTestSuite) TestReturnFix() {
	// register a function, returning float64
	ts.Nil(ts.trans.Register(
		"ReturnFix",
		func() float64 { return 0 },
	))

	// compose a task
	t, err := ts.trans.ComposeTask("ReturnFix", nil, nil)
	ts.Nil(err)

	// compose a corresponding report
	r, err := t.composeReport(Status.Success, []interface{}{int(6)}, nil)
	ts.Nil(err)

	// attach a reporting channel
	reports := make(chan *Report, 10)
	ts.Nil(ts.bg.Report(reports))

	// poll the task
	outputs, err := ts.bg.Poll(t)
	ts.Nil(err)

	reports <- r
	out, ok := <-outputs
	ts.True(ok)
	ts.Len(out.Return(), 1)
	if len(out.Return()) > 0 {
		v, ok := out.Return()[0].(float64)
		ts.True(ok)
		ts.Equal(float64(6), v)
	}
}
