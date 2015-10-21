package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/suite"
)

type DingoMonitorTestSuite struct {
	suite.Suite

	_id       string
	_count    int
	_mnt      *_monitors
	_store    backend.Store
	_reporter backend.Reporter
	_reports  chan meta.Report
	_invoker  meta.Invoker
}

func TestDingoMonitorSuite(t *testing.T) {
	suite.Run(t, &DingoMonitorTestSuite{
		_invoker: meta.NewDefaultInvoker(),
		_count:   3,
		_reports: make(chan meta.Report, 10),
	})
}

func (me *DingoMonitorTestSuite) SetupSuite() {
	var err error

	me._store, err = backend.New("local", nil)
	me.Nil(err)
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
		meta.Status.Sent,
		meta.Status.Progress,
		meta.Status.Test1,
		meta.Status.Test2,
		meta.Status.Test3,
		meta.Status.Test4,
		meta.Status.Test5,
		meta.Status.Test6,
	}
	me.Len(toSend, me._count+meta.Status.Count)
	for _, v := range toSend {
		// compose reports
		r, err := t.ComposeReport(v, []interface{}{}, nil)
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

func (me *DingoMonitorTestSuite) TestFitReturns() {
	me._mnt.register(&StrMatcher{"test_convert_return"}, func() (int, float32) {
		return 0, 0
	})

	// compose a task
	t, err := me._invoker.ComposeTask("test_convert_return")
	me.Nil(err)
	// register it to monitor's check list
	report, err := me._mnt.check(t)
	me.Nil(err)
	// send a report with different type (compared to function's
	// return value)
	r, err := t.ComposeReport(meta.Status.Done, []interface{}{
		int64(101),
		float32(5.2),
	}, nil)
	me.Nil(err)
	me._reports <- r
	// check returns
	for {
		r := <-report
		if r.Done() {
			ret := r.GetReturn()
			me.Len(ret, 2)
			me.Equal(int(101), ret[0])
			me.Equal(float32(5.2), ret[1])
			break
		}
	}
}
