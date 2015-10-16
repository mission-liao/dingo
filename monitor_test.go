package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/task"
	"github.com/stretchr/testify/suite"
)

type DingoMonitorTestSuite struct {
	suite.Suite

	_id       string
	_count    int
	_mnt      *_monitors
	_store    backend.Store
	_reporter backend.Reporter
	_reports  chan task.Report
	_invoker  task.Invoker
}

func TestDingoMonitorSuite(t *testing.T) {
	suite.Run(t, &DingoMonitorTestSuite{
		_invoker: task.NewDefaultInvoker(),
		_store:   backend.NewLocal(),
		_count:   3,
		_reports: make(chan task.Report, 10),
	})
}

func (me *DingoMonitorTestSuite) SetupSuite() {
	var err error
	me._mnt, err = newMonitors(me._store)
	me.Nil(err)
	me._mnt.more(me._count)

	me._reporter = me._store.(backend.Reporter)
	me._id, err = me._reporter.Report(me._reports)
	me.Nil(err)
}

func (me *DingoMonitorTestSuite) TearDownSuite() {
	me.Nil(me._mnt.done())
	me.Nil(me._reporter.Unbind(me._id))
	me.Nil(me._store.(common.Server).Close())
}

//
// test case
//

func (me *DingoMonitorTestSuite) TestParellenMonitoring() {
	// make sure other monitor routines would be
	// called when one is blocked.

	// compose a task
	t, err := me._invoker.ComposeTask("test")

	// add this task to monitors' check list
	report, err := me._mnt.check(t)
	me.NotNil(report)
	me.Nil(err)

	toSend := []int{
		task.Status.Sent,
		task.Status.Progress,
		task.Status.Test1,
		task.Status.Test2,
		task.Status.Test3,
		task.Status.Test4,
		task.Status.Test5,
		task.Status.Test6,
	}
	me.Len(toSend, me._count+task.Status.Count)
	for _, v := range toSend {
		// compose reports
		r, err := t.ComposeReport(v, nil, []interface{}{})
		me.Nil(err)

		if err != nil {
			continue
		}
		// send reports through backend.Reporter
		me._reports <- r
	}

	for _, v := range toSend {
		r := <-report
		me.Equal(v, r.GetStatus())
	}
}
