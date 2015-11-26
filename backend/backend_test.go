package backend

import (
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BackendTestSuite struct {
	suite.Suite

	_msh      *transport.Marshallers
	_backend  Backend
	_reporter Reporter
	_store    Store
	_reports  chan *Envelope
}

func (me *BackendTestSuite) SetupSuite() {
	var err error

	me.NotNil(me._backend)
	me._msh = transport.NewMarshallers()
	me._reports = make(chan *Envelope, 10)
	me._reporter, me._store = me._backend.(Reporter), me._backend.(Store)
	me.NotNil(me._reporter)
	me.NotNil(me._store)
	_, err = me._reporter.Report(me._reports)
	me.Nil(err)
}

func (me *BackendTestSuite) TearDownSuite() {
	me.Nil(me._backend.(common.Object).Close())
}

//
// test cases
//

func (me *BackendTestSuite) TestBasic() {
	// register an encoding for this method
	err := me._msh.Register("basic", func() {}, transport.Encode.Default, transport.Encode.Default)

	// compose a dummy task
	task, err := transport.ComposeTask("basic", []interface{}{})
	me.Nil(err)

	// send a report
	report, err := task.ComposeReport(transport.Status.Sent, make([]interface{}, 0), nil)
	me.Nil(err)
	{
		b, err := me._msh.EncodeReport(report)
		me.Nil(err)
		me._reports <- &Envelope{
			ID:   report,
			Body: b,
		}
	}

	// poll corresponding task
	reports, err := me._store.Poll(task)
	me.Nil(err)
	me.NotNil(reports)
	select {
	case v, ok := <-reports:
		me.True(ok)
		if !ok {
			break
		}
		r, err := me._msh.DecodeReport(v)
		me.Nil(err)
		me.True(report.Equal(r))
	}

	// done polling
	me.Nil(me._store.Done(task))
}
