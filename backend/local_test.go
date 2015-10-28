package backend

import (
	"testing"

	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

//
// Reporter
//

func TestLocalReporter(t *testing.T) {
	ass := assert.New(t)

	cfg := Default()
	cfg.Local.Bypass(false)

	v, err := New("local", cfg)
	reporter := v.(Reporter)

	// test case for Report/Unbind
	reports := make(chan meta.Report, 10)
	err = reporter.Report(reports)
	ass.Nil(err)
	err = reporter.Unbind()
	ass.Nil(err)

	// teardown
	v.(*_local).Close()
}

//
// Store
//

type LocalStoreTestSuite struct {
	suite.Suite

	_invoker  meta.Invoker
	_task     meta.Task
	_inst     interface{}
	_reporter Reporter
	_reports  chan meta.Report
	_store    Store
}

func (me *LocalStoreTestSuite) SetupSuite() {
	var (
		err error
	)

	me._invoker = meta.NewDefaultInvoker()
	me._task, err = me._invoker.ComposeTask("test", 123, "the string")
	me.Nil(err)

	cfg := Default()
	cfg.Local.Bypass(false)

	me._inst, err = New("local", cfg)
	me.Nil(err)
	me._reporter, me._store = me._inst.(Reporter), me._inst.(Store)
	me.NotNil(me._reporter)
	me.NotNil(me._store)
	me._reports = make(chan meta.Report, 10)
	me.Nil(me._reporter.Report(me._reports))
}

func (me *LocalStoreTestSuite) TearDownSuite() {
	me.Nil(me._reporter.Unbind())
	me._inst.(*_local).Close()
}

func (me *LocalStoreTestSuite) TestBasic() {
	// send a report
	report, err := me._task.ComposeReport(meta.Status.Sent, make([]interface{}, 0), nil)
	me.Nil(err)
	me._reports <- report

	// poll corresponding task
	me.Nil(me._store.Poll(me._task))
	reports, err := me._store.Subscribe()
	me.Nil(err)
	select {
	case v, ok := <-reports:
		me.True(ok)
		me.True(report.Identical(v))
	}

	// done polling
	me.Nil(me._store.Done(me._task))
}

func TestLocalStoreSuite(t *testing.T) {
	suite.Run(t, &LocalStoreTestSuite{
		_reports: make(chan meta.Report, 10),
	})
}

//
// Backend generic test cases
//

type LocalBackendTestSuite struct {
	BackendTestSuite

	bypass bool
}

func (me *LocalBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	cfg.Local.Bypass(me.bypass)
	me._backend, err = New("local", cfg)
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func (me *LocalBackendTestSuite) TearDownSuite() {
	me.BackendTestSuite.TearDownSuite()
}

func TestLocalBackendSuite(t *testing.T) {
	suite.Run(t, &LocalBackendTestSuite{bypass: true})
	suite.Run(t, &LocalBackendTestSuite{bypass: false})
}
