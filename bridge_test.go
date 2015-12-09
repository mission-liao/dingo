package dingo

import (
	"fmt"

	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BridgeTestSuite struct {
	suite.Suite

	name     string
	bg       Bridge
	trans    *transport.Mgr
	events   []*common.Event
	eventMux *common.Mux
}

func (me *BridgeTestSuite) SetupSuite() {
	me.eventMux = common.NewMux()
	me.trans = transport.NewMgr()
}

func (me *BridgeTestSuite) TearDownSuite() {
}

func (me *BridgeTestSuite) SetupTest() {
	// prepare Bridge
	me.bg = NewBridge(me.name, me.trans)

	// bind event channel
	es, err := me.bg.Events()
	me.Nil(err)
	for _, v := range es {
		_, err := me.eventMux.Register(v, 0)
		me.Nil(err)
	}

	// reset event array
	me.events = make([]*common.Event, 0, 10)
	me.eventMux.Handle(func(val interface{}, _ int) {
		me.events = append(me.events, val.(*common.Event))
	})
}

func (me *BridgeTestSuite) TearDownTest() {
	me.Nil(me.bg.Close())

	for _, v := range me.events {
		if v.Level == common.ErrLvl.ERROR {
			me.Nil(v)
		}
	}

	me.eventMux.Close()
}

//
// test cases
//

func (me *BridgeTestSuite) _TestSendTask() {
	me.trans.Register(
		"SendTask",
		func() {},
		transport.Encode.Default, transport.Encode.Default,
	)

	// add listener
	receipts := make(chan *broker.Receipt, 0)
	tasks, err := me.bg.AddListener(receipts)
	me.Nil(err)

	// compose a task
	t, err := transport.ComposeTask("SendTask", nil, nil)
	me.Nil(err)

	// send that task
	me.Nil(me.bg.SendTask(t))

	// make sure task is received through listeners
	tReceived := <-tasks
	me.Equal(t, tReceived)

	// send a receipt withoug blocked,
	// which means someone is waiting
	receipts <- &broker.Receipt{
		ID:     t.ID(),
		Status: broker.Status.OK,
	}
}

func (me *BridgeTestSuite) _TestAddListener() {
	me.trans.Register(
		"AddListener",
		func() {},
		transport.Encode.Default, transport.Encode.Default,
	)

	// prepare listeners
	r1 := make(chan *broker.Receipt, 0)
	t1, err := me.bg.AddListener(r1)
	me.Nil(err)
	r2 := make(chan *broker.Receipt, 0)
	t2, err := me.bg.AddListener(r2)
	me.Nil(err)

	r3 := make(chan *broker.Receipt, 0)
	t3, err := me.bg.AddListener(r3)
	me.Nil(err)

	// compose a task, and send it
	t, err := transport.ComposeTask("AddListener", nil, nil)
	me.Nil(err)
	me.Nil(me.bg.SendTask(t))

	// get that task from one channel
	pass := false
	var (
		tReceived *transport.Task
		receipt   *broker.Receipt = &broker.Receipt{
			ID:     t.ID(),
			Status: broker.Status.OK,
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
	me.Equal(t, tReceived)

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

func (me *BridgeTestSuite) _TestReport() {
	me.Nil(me.trans.Register(
		"Report",
		func() {},
		transport.Encode.Default, transport.Encode.Default,
	))

	// attach reporter channel
	reports := make(chan *transport.Report, 10)
	me.Nil(me.bg.Report(reports))

	// a sample task
	t, err := transport.ComposeTask("Report", nil, nil)
	me.Nil(err)
	outputs, err := me.bg.Poll(t)
	me.Nil(err)

	// normal report
	{
		input, err := t.ComposeReport(transport.Status.Sent, nil, nil)
		me.Nil(err)
		reports <- input
		r, ok := <-outputs
		me.True(ok)
		me.Equal(input.ID(), r.ID())
	}

	// a report with 'Done' == true
	// the output channel should be closed
	{
		input, err := t.ComposeReport(transport.Status.Done, nil, nil)
		me.Nil(err)
		reports <- input
		r, ok := <-outputs
		me.True(ok)
		me.Equal(input.ID(), r.ID())
		_, ok = <-outputs
		me.False(ok)
	}
}

func (me *BridgeTestSuite) _TestPoll() {
	me.Nil(me.trans.Register(
		"Poll",
		func() {},
		transport.Encode.Default, transport.Encode.Default,
	))
	count := 1

	// multiple reports channel
	rs := []chan *transport.Report{}
	for i := 0; i < count; i++ {
		rs = append(rs, make(chan *transport.Report, 10))
		me.Nil(me.bg.Report(rs[len(rs)-1]))
	}

	// multiple tasks
	// - t1, t2 -> r1
	// - t3, t4 -> r2
	// - t5 -> r3
	ts := []*transport.Task{}
	for i := 0; i < count; i++ {
		t, err := transport.ComposeTask("Poll", nil, []interface{}{fmt.Sprintf("t%d", 2*i)})
		me.Nil(err)
		ts = append(ts, t)
		t, err = transport.ComposeTask("Poll", nil, []interface{}{fmt.Sprintf("t%d", 2*i+1)})
		me.Nil(err)
		ts = append(ts, t)
	}

	// send reports through those channels,
	// reports belongs to one task should be sent
	// to the same reporter
	for k, t := range ts {
		r, err := t.ComposeReport(transport.Status.Sent, nil, nil)
		me.Nil(err)
		rs[k/2] <- r

		r, err = t.ComposeReport(transport.Status.Progress, nil, nil)
		me.Nil(err)
		rs[k/2] <- r

		r, err = t.ComposeReport(transport.Status.Done, nil, nil)
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
		me.Equal(transport.Status.Sent, r.Status())
		r, ok = <-out
		me.True(ok)
		me.Equal(transport.Status.Progress, r.Status())
		r, ok = <-out
		me.True(ok)
		me.Equal(transport.Status.Done, r.Status())
		r, ok = <-out
		me.False(ok)
	}
}

func (me *BridgeTestSuite) TestExist() {
	me.True(me.bg.Exists(common.InstT.REPORTER))
	me.True(me.bg.Exists(common.InstT.CONSUMER))
	me.True(me.bg.Exists(common.InstT.PRODUCER))
	me.True(me.bg.Exists(common.InstT.STORE))

	// checking should be ==, not &=
	me.False(me.bg.Exists(common.InstT.STORE | common.InstT.REPORTER))
	me.False(me.bg.Exists(common.InstT.ALL))
}
