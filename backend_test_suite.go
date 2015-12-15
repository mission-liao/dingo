package dingo

import (
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BackendTestSuite struct {
	suite.Suite

	Trans   *transport.Mgr
	Bkd     Backend
	Rpt     Reporter
	Sto     Store
	Reports chan *ReportEnvelope
	Task    *transport.Task
}

func (me *BackendTestSuite) SetupSuite() {
	var err error

	me.NotNil(me.Bkd)
	me.Trans = transport.NewMgr()
	me.Reports = make(chan *ReportEnvelope, 10)
	me.Rpt, me.Sto = me.Bkd.(Reporter), me.Bkd.(Store)
	me.NotNil(me.Rpt)
	me.NotNil(me.Sto)
	_, err = me.Rpt.Report(me.Reports)
	me.Nil(err)
}

func (me *BackendTestSuite) TearDownSuite() {
	me.Nil(me.Bkd.(common.Object).Close())
}

//
// test cases
//

func (me *BackendTestSuite) TestBasic() {
	// register an encoding for this method
	err := me.Trans.Register("basic", func() {}, transport.Encode.Default, transport.Encode.Default)

	// compose a dummy task
	me.Task, err = transport.ComposeTask("basic", nil, []interface{}{})
	me.Nil(err)

	// poll first
	reports, err := me.Sto.Poll(me.Task)
	me.Nil(err)
	me.NotNil(reports)

	// send a report
	report, err := me.Task.ComposeReport(transport.Status.Sent, make([]interface{}, 0), nil)
	me.Nil(err)
	{
		b, err := me.Trans.EncodeReport(report)
		me.Nil(err)
		me.Reports <- &ReportEnvelope{
			ID:   report,
			Body: b,
		}
	}

	// await it
	select {
	case v, ok := <-reports:
		me.True(ok)
		if !ok {
			break
		}
		r, err := me.Trans.DecodeReport(v)
		me.Nil(err)
		me.True(report.Equal(r))
	}

	// done polling
	me.Nil(me.Sto.Done(me.Task))
}
