package dingo

import (
	"fmt"
	"sync"

	"github.com/stretchr/testify/suite"
)

type BridgeTestSuite struct {
	suite.Suite

	name     string
	bg       bridge
	trans    *mgr
	events   []*Event
	eventMux *mux
}

func (me *BridgeTestSuite) SetupSuite() {
	me.eventMux = newMux()
	me.trans = newMgr()
}

func (me *BridgeTestSuite) TearDownSuite() {
}

func (me *BridgeTestSuite) SetupTest() {
	// prepare Bridge
	me.bg = newBridge(me.name, me.trans)

	// bind event channel
	es, err := me.bg.Events()
	me.Nil(err)
	for _, v := range es {
		_, err := me.eventMux.Register(v, 0)
		me.Nil(err)
	}

	// reset event array
	me.events = make([]*Event, 0, 10)
	me.eventMux.Handle(func(val interface{}, _ int) {
		me.events = append(me.events, val.(*Event))
	})
}

func (me *BridgeTestSuite) TearDownTest() {
	me.Nil(me.bg.Close())

	for _, v := range me.events {
		if v.Level == EventLvl.ERROR {
			me.Nil(v)
		}
	}

	me.eventMux.Close()
}

func (me *BridgeTestSuite) send(reports chan<- *Report, task *Task, s int16) {
	r, err := task.ComposeReport(s, nil, nil)
	me.Nil(err)

	reports <- r
}

func (me *BridgeTestSuite) chk(expected *Task, got *Report, s int16) {
	me.Equal(expected.ID(), got.ID())
	me.Equal(expected.Name(), got.Name())
	me.Equal(s, got.Status())
}

func (me *BridgeTestSuite) gen(reports chan<- *Report, task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	me.Nil(me.bg.(exHooks).ReporterHook(ReporterEvent.BeforeReport, task))

	me.send(reports, task, Status.Sent)
	me.send(reports, task, Status.Progress)
	me.send(reports, task, Status.Success)
}

func (me *BridgeTestSuite) chks(task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	r, err := me.bg.Poll(task)
	me.Nil(err)

	me.chk(task, <-r, Status.Sent)
	me.chk(task, <-r, Status.Progress)
	me.chk(task, <-r, Status.Success)
}

//
// test cases
//

func (me *BridgeTestSuite) TestSendTask() {
	me.trans.Register(
		"SendTask",
		func() {},
		Encode.Default, Encode.Default, ID.Default,
	)

	// add listener
	receipts := make(chan *TaskReceipt, 0)
	tasks, err := me.bg.AddListener(receipts)
	me.Nil(err)

	// compose a task
	t, err := me.trans.ComposeTask("SendTask", nil, nil)
	me.Nil(err)

	// send that task
	me.Nil(me.bg.SendTask(t))

	// make sure task is received through listeners
	tReceived := <-tasks
	me.True(t.AlmostEqual(tReceived))

	// send a receipt withoug blocked,
	// which means someone is waiting
	receipts <- &TaskReceipt{
		ID:     t.ID(),
		Status: ReceiptStatus.OK,
	}
}

func (me *BridgeTestSuite) TestAddListener() {
	me.trans.Register(
		"AddListener",
		func() {},
		Encode.Default, Encode.Default, ID.Default,
	)

	// prepare listeners
	r1 := make(chan *TaskReceipt, 0)
	t1, err := me.bg.AddListener(r1)
	me.Nil(err)
	r2 := make(chan *TaskReceipt, 0)
	t2, err := me.bg.AddListener(r2)
	me.Nil(err)

	r3 := make(chan *TaskReceipt, 0)
	t3, err := me.bg.AddListener(r3)
	me.Nil(err)

	// compose a task, and send it
	t, err := me.trans.ComposeTask("AddListener", nil, nil)
	me.Nil(err)
	me.Nil(me.bg.SendTask(t))

	// get that task from one channel
	pass := false
	var (
		tReceived *Task
		receipt   *TaskReceipt = &TaskReceipt{
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
	me.True(pass)
	me.True(t.AlmostEqual(tReceived))

	// close all listeners
	me.Nil(me.bg.StopAllListeners())

	// task channels are closed
	_, ok := <-t1
	me.False(ok)
	_, ok = <-t2
	me.False(ok)
	_, ok = <-t3
	me.False(ok)
}

func (me *BridgeTestSuite) TestMultipleClose() {
	me.Nil(me.bg.Close())
	me.Nil(me.bg.Close())
}

func (me *BridgeTestSuite) TestReport() {
	me.Nil(me.trans.Register(
		"Report",
		func() {},
		Encode.Default, Encode.Default, ID.Default,
	))

	// attach reporter channel
	reports := make(chan *Report, 10)
	me.Nil(me.bg.Report(reports))

	// a sample task
	t, err := me.trans.ComposeTask("Report", nil, nil)
	me.Nil(err)
	outputs, err := me.bg.Poll(t)
	me.Nil(err)

	// normal report
	{
		input, err := t.ComposeReport(Status.Sent, nil, nil)
		me.Nil(err)
		reports <- input
		r, ok := <-outputs
		me.True(ok)
		me.Equal(input.ID(), r.ID())
	}

	// a report with 'Done' == true
	// the output channel should be closed
	{
		input, err := t.ComposeReport(Status.Success, nil, nil)
		me.Nil(err)
		reports <- input
		r, ok := <-outputs
		me.True(ok)
		me.Equal(input.ID(), r.ID())
		_, ok = <-outputs
		me.False(ok)
	}
}

func (me *BridgeTestSuite) TestPoll() {
	me.Nil(me.trans.Register(
		"Poll",
		func() {},
		Encode.Default, Encode.Default, ID.Default,
	))
	count := 1

	// multiple reports channel
	rs := []chan *Report{}
	for i := 0; i < count; i++ {
		rs = append(rs, make(chan *Report, 10))
		me.Nil(me.bg.Report(rs[len(rs)-1]))
	}

	// multiple tasks
	// - t1, t2 -> r1
	// - t3, t4 -> r2
	// - t5 -> r3
	ts := []*Task{}
	for i := 0; i < count; i++ {
		t, err := me.trans.ComposeTask("Poll", nil, []interface{}{fmt.Sprintf("t%d", 2*i)})
		me.Nil(err)
		ts = append(ts, t)
		t, err = me.trans.ComposeTask("Poll", nil, []interface{}{fmt.Sprintf("t%d", 2*i+1)})
		me.Nil(err)
		ts = append(ts, t)
	}

	// send reports through those channels,
	// reports belongs to one task should be sent
	// to the same reporter
	for k, t := range ts {
		r, err := t.ComposeReport(Status.Sent, nil, nil)
		me.Nil(err)
		rs[k/2] <- r

		r, err = t.ComposeReport(Status.Progress, nil, nil)
		me.Nil(err)
		rs[k/2] <- r

		r, err = t.ComposeReport(Status.Success, nil, nil)
		me.Nil(err)
		rs[k/2] <- r
	}

	// each poller should get expected reports,
	// nothing more than that.
	for _, t := range ts {
		out, err := me.bg.Poll(t)
		me.Nil(err)
		if err != nil {
			break
		}

		r, ok := <-out
		me.True(ok)
		me.Equal(Status.Sent, r.Status())
		r, ok = <-out
		me.True(ok)
		me.Equal(Status.Progress, r.Status())
		r, ok = <-out
		me.True(ok)
		me.Equal(Status.Success, r.Status())
		r, ok = <-out
		me.False(ok)
	}
}

func (me *BridgeTestSuite) TestExist() {
	me.True(me.bg.Exists(InstT.REPORTER))
	me.True(me.bg.Exists(InstT.CONSUMER))
	me.True(me.bg.Exists(InstT.PRODUCER))
	me.True(me.bg.Exists(InstT.STORE))

	// checking should be ==, not &=
	me.False(me.bg.Exists(InstT.STORE | InstT.REPORTER))
	me.False(me.bg.Exists(InstT.ALL))
}

func (me *BridgeTestSuite) TestFinalReportWhenShutdown() {
	// when exiting, remaining polling should be closed,
	// and a final report with

	// register a marshaller
	me.Nil(me.trans.Register(
		"FinalReportWhenShutdown",
		func() {},
		Encode.Default, Encode.Default, ID.Default,
	))

	// a report channel
	reports := make(chan *Report, 1)
	me.Nil(me.bg.Report(reports))

	// a sample task
	task, err := me.trans.ComposeTask("FinalReportWhenShutdown", nil, nil)

	// poll that task
	out, err := me.bg.Poll(task)
	me.Nil(err)

	// send reports: Sent, but not the final one
	r, err := task.ComposeReport(Status.Sent, nil, nil)
	me.Nil(err)
	reports <- r
	o := <-out
	me.True(o.AlmostEqual(r))

	// close bridge, a 'Shutdown' report should be received.
	me.Nil(me.bg.Close())
	o = <-out
	me.Equal(Status.Fail, o.Status())
	me.Equal(ErrCode.Shutdown, o.Error().Code())
}

func (me *BridgeTestSuite) TestDifferentReportsWithSameID() {
	// bridge should be ok when different reports have the same ID
	var (
		countOfTypes int = 10
		countOfTasks int = 10
		tasks        []*Task
		wait         sync.WaitGroup
	)

	reports := make(chan *Report, 10)
	me.Nil(me.bg.Report(reports))

	// register idMaker, task
	for i := 0; i < countOfTypes; i++ {
		name := fmt.Sprintf("DifferentReportsWithSameID.%d", i)
		me.Nil(me.trans.AddIdMaker(100+i, &testSeqID{}))
		me.Nil(me.trans.Register(name, func() {}, Encode.Default, Encode.Default, 100+i))

		for j := 0; j < countOfTasks; j++ {
			t, err := me.trans.ComposeTask(name, nil, nil)
			me.Nil(err)
			if t != nil {
				wait.Add(1)
				go me.gen(reports, t, &wait)

				tasks = append(tasks, t)
			}
		}
	}

	// wait for all routines finished
	wait.Wait()

	for _, v := range tasks {
		wait.Add(1)
		go me.chks(v, &wait)
	}
	// wait for all chks routine
	wait.Wait()
}
