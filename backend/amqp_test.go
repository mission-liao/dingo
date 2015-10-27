package backend

import (
	"testing"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/suite"
)

type AmqpBackendTestSuite struct {
	suite.Suite

	_invoker  meta.Invoker
	_backend  Backend
	_reporter Reporter
	_store    Store
	_reports  chan meta.Report
}

func (me *AmqpBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	me._backend, err = New("amqp", cfg)
	me.NotNil(me._backend)
	me.Nil(err)
	me._reporter, me._store = me._backend.(Reporter), me._backend.(Store)
	me.Nil(me._reporter.Report(me._reports))
}

func (me *AmqpBackendTestSuite) TearDownSuite() {
	me.Nil(me._reporter.Unbind())
	me.Nil(me._backend.(common.Server).Close())
}

func TestAmqpBackendSuite(t *testing.T) {
	suite.Run(t, &AmqpBackendTestSuite{
		_reports: make(chan meta.Report, 10),
		_invoker: meta.NewDefaultInvoker(),
	})
}

//
// test cases
//

func (me *AmqpBackendTestSuite) TestBasic() {
	// compose a dummy task
	task, err := me._invoker.ComposeTask("test")
	me.Nil(err)

	// send a report
	report, err := task.ComposeReport(meta.Status.Sent, make([]interface{}, 0), nil)
	me.Nil(err)
	me._reports <- report

	// poll corresponding task
	me.Nil(me._store.Poll(task))
	reports, err := me._store.Subscribe()
	me.Nil(err)
	select {
	case v, ok := <-reports:
		me.True(ok)
		me.True(report.Identical(v))
	}

	// done polling
	me.Nil(me._store.Done(task))
}
