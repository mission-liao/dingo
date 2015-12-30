package dingo

import (
	"fmt"
	"sync"

	"github.com/stretchr/testify/suite"
)

type bridgeTestSuite struct {
	suite.Suite

	name     string
	bg       bridge
	trans    *fnMgr
	events   []*Event
	eventMux *mux
}

func (ts *bridgeTestSuite) SetupSuite() {
	ts.eventMux = newMux()
	ts.trans = newFnMgr()
}

func (ts *bridgeTestSuite) TearDownSuite() {
}

func (ts *bridgeTestSuite) SetupTest() {
	// prepare Bridge
	ts.bg = newBridge(ts.name, ts.trans)

	// bind event channel
	es, err := ts.bg.Events()
	ts.Nil(err)
	for _, v := range es {
		_, err := ts.eventMux.Register(v, 0)
		ts.Nil(err)
	}

	// reset event array
	ts.events = make([]*Event, 0, 10)
	ts.eventMux.Handle(func(val interface{}, _ int) {
		ts.events = append(ts.events, val.(*Event))
	})
}

func (ts *bridgeTestSuite) TearDownTest() {
	ts.Nil(ts.bg.Close())

	for _, v := range ts.events {
		if v.Level == EventLvl.Error {
			ts.Nil(v)
		}
	}

	ts.eventMux.Close()
}

func (ts *bridgeTestSuite) send(reports chan<- *Report, task *Task, s int16) {
	r, err := task.composeReport(s, nil, nil)
	ts.Nil(err)

	reports <- r
}

func (ts *bridgeTestSuite) chk(expected *Task, got *Report, s int16) {
	ts.Equal(expected.ID(), got.ID())
	ts.Equal(expected.Name(), got.Name())
	ts.Equal(s, got.Status())
}

func (ts *bridgeTestSuite) gen(reports chan<- *Report, task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	ts.Nil(ts.bg.(exHooks).ReporterHook(ReporterEvent.BeforeReport, task))

	ts.send(reports, task, Status.Sent)
	ts.send(reports, task, Status.Progress)
	ts.send(reports, task, Status.Success)
}

func (ts *bridgeTestSuite) chks(task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	r, err := ts.bg.Poll(task)
	ts.Nil(err)

	ts.chk(task, <-r, Status.Sent)
	ts.chk(task, <-r, Status.Progress)
	ts.chk(task, <-r, Status.Success)
}

//
// test cases
//

func (ts *bridgeTestSuite) TestSendTask() {
	ts.trans.Register(
		"SendTask",
		func() {},
	)

	// add listener
	receipts := make(chan *TaskReceipt, 0)
	tasks, err := ts.bg.AddListener(receipts)
	ts.Nil(err)

	// compose a task
	t, err := ts.trans.ComposeTask("SendTask", nil, nil)
	ts.Nil(err)

	// send that task
	ts.Nil(ts.bg.SendTask(t))

	// make sure task is received through listeners
	tReceived := <-tasks
	ts.True(t.almostEqual(tReceived))

	// send a receipt withoug blocked,
	// which means someone is waiting
	receipts <- &TaskReceipt{
		ID:     t.ID(),
		Status: ReceiptStatus.OK,
	}
}

func (ts *bridgeTestSuite) TestAddListener() {
	ts.trans.Register(
		"AddListener",
		func() {},
	)

	// prepare listeners
	r1 := make(chan *TaskReceipt, 0)
	t1, err := ts.bg.AddListener(r1)
	ts.Nil(err)
	r2 := make(chan *TaskReceipt, 0)
	t2, err := ts.bg.AddListener(r2)
	ts.Nil(err)

	r3 := make(chan *TaskReceipt, 0)
	t3, err := ts.bg.AddListener(r3)
	ts.Nil(err)

	// compose a task, and send it
	t, err := ts.trans.ComposeTask("AddListener", nil, nil)
	ts.Nil(err)
	ts.Nil(ts.bg.SendTask(t))

	// get that task from one channel
	pass := false
	var (
		tReceived *Task
		receipt   = &TaskReceipt{
			ID:     t.ID(),
			Status: ReceiptStatus.OK,
		}
	)
	select {
	case tReceived = <-t1:
		r1 <- receipt
		pass = true
	case tReceived = <-t2:
		r2 <- receipt
		pass = true
	case tReceived = <-t3:
		r3 <- receipt
		pass = true
	}
	ts.True(pass)
	ts.True(t.almostEqual(tReceived))

	// close all listeners
	ts.Nil(ts.bg.StopAllListeners())

	// task channels are closed
	_, ok := <-t1
	ts.False(ok)
	_, ok = <-t2
	ts.False(ok)
	_, ok = <-t3
	ts.False(ok)
}

func (ts *bridgeTestSuite) TestMultipleClose() {
	ts.Nil(ts.bg.Close())
	ts.Nil(ts.bg.Close())
}

func (ts *bridgeTestSuite) TestReport() {
	ts.Nil(ts.trans.Register(
		"Report",
		func() {},
	))

	// attach reporter channel
	reports := make(chan *Report, 10)
	ts.Nil(ts.bg.Report("Report", reports))

	// a sample task
	t, err := ts.trans.ComposeTask("Report", nil, nil)
	ts.Nil(err)
	outputs, err := ts.bg.Poll(t)
	ts.Nil(err)

	// normal report
	{
		input, err := t.composeReport(Status.Sent, nil, nil)
		ts.Nil(err)
		reports <- input
		r, ok := <-outputs
		ts.True(ok)
		ts.Equal(input.ID(), r.ID())
	}

	// a report with 'Done' == true
	// the output channel should be closed
	{
		input, err := t.composeReport(Status.Success, nil, nil)
		ts.Nil(err)
		reports <- input
		r, ok := <-outputs
		ts.True(ok)
		ts.Equal(input.ID(), r.ID())
		_, ok = <-outputs
		ts.False(ok)
	}
}

func (ts *bridgeTestSuite) TestPoll() {
	ts.Nil(ts.trans.Register(
		"Poll",
		func() {},
	))
	count := 1

	// multiple reports channel
	rs := []chan *Report{}
	for i := 0; i < count; i++ {
		rs = append(rs, make(chan *Report, 10))
		ts.Nil(ts.bg.Report("Poll", rs[len(rs)-1]))
	}

	// multiple tasks
	// - t1, t2 -> r1
	// - t3, t4 -> r2
	// - t5 -> r3
	tasks := []*Task{}
	for i := 0; i < count; i++ {
		t, err := ts.trans.ComposeTask("Poll", nil, []interface{}{fmt.Sprintf("t%d", 2*i)})
		ts.Nil(err)
		tasks = append(tasks, t)
		t, err = ts.trans.ComposeTask("Poll", nil, []interface{}{fmt.Sprintf("t%d", 2*i+1)})
		ts.Nil(err)
		tasks = append(tasks, t)
	}

	// send reports through those channels,
	// reports belongs to one task should be sent
	// to the same reporter
	for k, t := range tasks {
		r, err := t.composeReport(Status.Sent, nil, nil)
		ts.Nil(err)
		rs[k/2] <- r

		r, err = t.composeReport(Status.Progress, nil, nil)
		ts.Nil(err)
		rs[k/2] <- r

		r, err = t.composeReport(Status.Success, nil, nil)
		ts.Nil(err)
		rs[k/2] <- r
	}

	// each poller should get expected reports,
	// nothing more than that.
	for _, t := range tasks {
		out, err := ts.bg.Poll(t)
		ts.Nil(err)
		if err != nil {
			break
		}

		r, ok := <-out
		ts.True(ok)
		ts.Equal(Status.Sent, r.Status())
		r, ok = <-out
		ts.True(ok)
		ts.Equal(Status.Progress, r.Status())
		r, ok = <-out
		ts.True(ok)
		ts.Equal(Status.Success, r.Status())
		r, ok = <-out
		ts.False(ok)
	}
}

func (ts *bridgeTestSuite) TestExist() {
	ts.True(ts.bg.Exists(ObjT.Reporter))
	ts.True(ts.bg.Exists(ObjT.Consumer))
	ts.True(ts.bg.Exists(ObjT.Producer))
	ts.True(ts.bg.Exists(ObjT.Store))

	// checking should be ==, not &=
	ts.False(ts.bg.Exists(ObjT.Store | ObjT.Reporter))
	ts.False(ts.bg.Exists(ObjT.All))
}

func (ts *bridgeTestSuite) TestFinalReportWhenShutdown() {
	// when exiting, remaining polling should be closed,
	// and a final report with

	// register a marshaller
	ts.Nil(ts.trans.Register(
		"FinalReportWhenShutdown",
		func() {},
	))

	// a report channel
	reports := make(chan *Report, 1)
	ts.Nil(ts.bg.Report("FinalReportWhenShutdown", reports))

	// a sample task
	task, err := ts.trans.ComposeTask("FinalReportWhenShutdown", nil, nil)

	// poll that task
	out, err := ts.bg.Poll(task)
	ts.Nil(err)

	// send reports: Sent, but not the final one
	r, err := task.composeReport(Status.Sent, nil, nil)
	ts.Nil(err)
	reports <- r
	o := <-out
	ts.True(o.almostEqual(r))

	// close bridge, a 'Shutdown' report should be received.
	ts.Nil(ts.bg.Close())
	o = <-out
	ts.Equal(Status.Fail, o.Status())
	ts.Equal(ErrCode.Shutdown, o.Error().Code())
}

func (ts *bridgeTestSuite) TestDifferentReportsWithSameID() {
	// bridge should be ok when different reports have the same ID
	var (
		countOfTypes = 10
		countOfTasks = 10
		tasks        []*Task
		t            *Task
		wait         sync.WaitGroup
		err          error
	)
	defer func() {
		ts.Nil(err)
	}()

	// register idMaker, task
	for i := 0; i < countOfTypes; i++ {
		name := fmt.Sprintf("DifferentReportsWithSameID.%d", i)
		err = ts.trans.AddIDMaker(100+i, &testSeqID{})
		if err != nil {
			return
		}
		err = ts.trans.Register(name, func() {})
		if err != nil {
			return
		}
		err = ts.trans.SetIDMaker(name, 100+i)
		if err != nil {
			return
		}

		reports := make(chan *Report, 10)
		err = ts.bg.Report(name, reports)
		if err != nil {
			return
		}
		for j := 0; j < countOfTasks; j++ {
			t, err = ts.trans.ComposeTask(name, nil, nil)
			if err != nil {
				return
			}
			if t != nil {
				wait.Add(1)
				go ts.gen(reports, t, &wait)

				tasks = append(tasks, t)
			}
		}
	}

	// wait for all routines finished
	wait.Wait()

	for _, v := range tasks {
		wait.Add(1)
		go ts.chks(v, &wait)
	}
	// wait for all chks routine
	wait.Wait()
}

func (ts *bridgeTestSuite) TestAddNamedListener() {
	var err error
	defer func() {
		ts.Nil(err)
	}()

	// register a task
	err = ts.trans.Register("TestAddNamedListener", func() {})
	if err != nil {
		return
	}

	// add a named listener, should fail
	tasks, err2 := ts.bg.AddNamedListener("TestAddNamedListener", make(chan *TaskReceipt, 10))
	ts.Nil(tasks)
	ts.NotNil(err2)
}
