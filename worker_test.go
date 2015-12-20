package dingo

import (
	"sort"
	"testing"
	"time"

	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type workerTestSuite struct {
	suite.Suite

	_ws    *_workers
	_trans *transport.Mgr
	_hooks exHooks
}

func TestWorkerSuite(t *testing.T) {
	suite.Run(t, &workerTestSuite{
		_trans: transport.NewMgr(),
		_hooks: newLocalBridge().(exHooks),
	})
}

func (me *workerTestSuite) SetupSuite() {
	var err error
	me._ws, err = newWorkers(me._trans, me._hooks)
	me.Nil(err)
}

func (me *workerTestSuite) TearDownSuite() {
	me.Nil(me._ws.Close())
}

//
// test cases
//

func (me *workerTestSuite) TestParellelRun() {
	// make sure other workers would be called
	// when one is blocked.

	stepIn := make(chan int, 3)
	stepOut := make(chan int)
	tasks := make(chan *transport.Task)
	fn := func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}
	me.Nil(me._trans.Register(
		"TestParellelRun", fn,
		transport.Encode.Default, transport.Encode.Default, transport.ID.Default,
	))
	reports, remain, err := me._ws.allocate("TestParellelRun", tasks, nil, 3, 0)
	me.Nil(err)
	me.Equal(0, remain)
	me.Len(reports, 1)

	for i := 0; i < 3; i++ {
		t, err := me._trans.ComposeTask("TestParellelRun", nil, []interface{}{i})
		me.Nil(err)
		if err == nil {
			tasks <- t
		}
	}

	rets := []int{}
	for i := 0; i < 3; i++ {
		rets = append(rets, <-stepIn)
	}
	sort.Ints(rets)
	me.Equal([]int{0, 1, 2}, rets)

	stepOut <- 1
	stepOut <- 1
	stepOut <- 1
	close(stepIn)
	close(stepOut)
}

func (me *workerTestSuite) TestPanic() {
	// allocate workers
	tasks := make(chan *transport.Task)
	me.Nil(me._trans.Register("TestPanic", func() { panic("QQ") }, transport.Encode.Default, transport.Encode.Default, transport.ID.Default))
	reports, remain, err := me._ws.allocate("TestPanic", tasks, nil, 1, 0)
	me.Nil(err)
	me.Equal(0, remain)
	me.Len(reports, 1)

	// an option with MonitorProgress == false
	task, err := me._trans.ComposeTask("TestPanic", transport.NewOption(), nil)
	me.NotNil(task)
	me.Nil(err)
	if task != nil {
		// sending a task
		tasks <- task
		// await for reports
		r := <-reports[0]
		// should be a failed one
		me.True(r.Fail())
		me.Equal(transport.ErrCode.Panic, r.Error().Code())
	}
}

func (me *workerTestSuite) TestIgnoreReport() {
	// allocate workers
	tasks := make(chan *transport.Task)
	me.Nil(me._trans.Register("TestIgnoreReport", func() {}, transport.Encode.Default, transport.Encode.Default, transport.ID.Default))
	reports, remain, err := me._ws.allocate("TestIgnoreReport", tasks, nil, 1, 0)
	me.Nil(err)
	me.Equal(0, remain)
	me.Len(reports, 1)

	// an option with IgnoreReport == true
	task, err := me._trans.ComposeTask("TestIgnoreReport", transport.NewOption().SetIgnoreReport(true), nil)
	me.NotNil(task)
	me.Nil(err)

	// send task, and shouldn't get any report
	if task != nil {
		tasks <- task
		select {
		case <-reports[0]:
			me.Fail("shouldn't receive any reports")
		case <-time.After(500 * time.Millisecond):
			// wait for 0.5 second
		}
	}
}

func (me *workerTestSuite) TestMonitorProgress() {
	// allocate workers
	tasks := make(chan *transport.Task)
	me.Nil(me._trans.Register("TestOnlyResult", func() {}, transport.Encode.Default, transport.Encode.Default, transport.ID.Default))
	reports, remain, err := me._ws.allocate("TestOnlyResult", tasks, nil, 1, 0)
	me.Nil(err)
	me.Equal(0, remain)
	me.Len(reports, 1)

	// an option with MonitorProgress == false
	task, err := me._trans.ComposeTask("TestOnlyResult", transport.NewOption(), nil)
	me.NotNil(task)
	me.Nil(err)

	// send task, only the last report should be sent
	if task != nil {
		tasks <- task
		r := <-reports[0]
		me.True(r.Done())
	}
}
