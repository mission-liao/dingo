package dingo

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type workerTestSuite struct {
	suite.Suite

	_ws    *_workers
	_trans *fnMgr
	_hooks exHooks
}

func TestWorkerSuite(t *testing.T) {
	suite.Run(t, &workerTestSuite{
		_trans: newFnMgr(""),
		_hooks: newLocalBridge().(exHooks),
	})
}

func (ts *workerTestSuite) SetupSuite() {
	var err error
	ts._ws, err = newWorkers(ts._trans, ts._hooks)
	ts.Nil(err)
}

func (ts *workerTestSuite) TearDownSuite() {
	ts.Nil(ts._ws.Close())
}

//
// test cases
//

func (ts *workerTestSuite) TestParellelRun() {
	// make sure other workers would be called
	// when one is blocked.

	stepIn := make(chan int, 3)
	stepOut := make(chan int)
	tasks := make(chan *Task)
	fn := func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}
	ts.Nil(ts._trans.Register(
		"TestParellelRun", fn,
	))
	reports, remain, err := ts._ws.allocate("TestParellelRun", tasks, nil, 3, 0)
	ts.Nil(err)
	ts.Equal(0, remain)
	ts.Len(reports, 1)

	for i := 0; i < 3; i++ {
		t, err := ts._trans.ComposeTask("TestParellelRun", nil, []interface{}{i})
		ts.Nil(err)
		if err == nil {
			tasks <- t
		}
	}

	rets := []int{}
	for i := 0; i < 3; i++ {
		rets = append(rets, <-stepIn)
	}
	sort.Ints(rets)
	ts.Equal([]int{0, 1, 2}, rets)

	stepOut <- 1
	stepOut <- 1
	stepOut <- 1
	close(stepIn)
	close(stepOut)
}

func (ts *workerTestSuite) TestPanic() {
	// allocate workers
	tasks := make(chan *Task)
	ts.Nil(ts._trans.Register("TestPanic", func() { panic("QQ") }))
	reports, remain, err := ts._ws.allocate("TestPanic", tasks, nil, 1, 0)
	ts.Nil(err)
	ts.Equal(0, remain)
	ts.Len(reports, 1)

	// an option with MonitorProgress == false
	task, err := ts._trans.ComposeTask("TestPanic", DefaultOption(), nil)
	ts.NotNil(task)
	ts.Nil(err)
	if task != nil {
		// sending a task
		tasks <- task
		// await for reports
		r := <-reports[0]
		// should be a failed one
		ts.True(r.Fail())
		ts.Equal(ErrCode.Panic, r.Error().Code())
	}
}

func (ts *workerTestSuite) TestIgnoreReport() {
	// allocate workers
	tasks := make(chan *Task)
	ts.Nil(ts._trans.Register("TestIgnoreReport", func() {}))
	reports, remain, err := ts._ws.allocate("TestIgnoreReport", tasks, nil, 1, 0)
	ts.Nil(err)
	ts.Equal(0, remain)
	ts.Len(reports, 1)

	// an option with IgnoreReport == true
	task, err := ts._trans.ComposeTask("TestIgnoreReport", DefaultOption().IgnoreReport(true), nil)
	ts.NotNil(task)
	ts.Nil(err)

	// send task, and shouldn't get any report
	if task != nil {
		tasks <- task
		select {
		case <-reports[0]:
			ts.Fail("shouldn't receive any reports")
		case <-time.After(500 * time.Millisecond):
			// wait for 0.5 second
		}
	}
}

func (ts *workerTestSuite) TestMonitorProgress() {
	// allocate workers
	tasks := make(chan *Task)
	ts.Nil(ts._trans.Register("TestOnlyResult", func() {}))
	reports, remain, err := ts._ws.allocate("TestOnlyResult", tasks, nil, 1, 0)
	ts.Nil(err)
	ts.Equal(0, remain)
	ts.Len(reports, 1)

	// an option with MonitorProgress == false
	task, err := ts._trans.ComposeTask("TestOnlyResult", DefaultOption(), nil)
	ts.NotNil(task)
	ts.Nil(err)

	// send task, only the last report should be sent
	if task != nil {
		tasks <- task
		r := <-reports[0]
		ts.True(r.Done())
	}
}
