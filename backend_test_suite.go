package dingo

import (
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BackendTestSuite struct {
	suite.Suite

	Trans   *transport.Mgr
	Bkd     Backend
	Rpt     Reporter
	Sto     Store
	Reports chan *ReportEnvelope
	Task    *transport.Task
}

func (me *BackendTestSuite) SetupSuite() {
	var err error

	me.NotNil(me.Bkd)
	me.Trans = transport.NewMgr()
	me.Reports = make(chan *ReportEnvelope, 10)
	me.Rpt, me.Sto = me.Bkd.(Reporter), me.Bkd.(Store)
	me.NotNil(me.Rpt)
	me.NotNil(me.Sto)
	_, err = me.Rpt.Report(me.Reports)
	me.Nil(err)
}

func (me *BackendTestSuite) TearDownSuite() {
	me.Nil(me.Bkd.(common.Object).Close())
}

//
// test cases
//

func (me *BackendTestSuite) TestBasic() {
	var err error

	// register an encoding for this method
	me.Nil(me.Trans.Register("basic", func() {}, transport.Encode.Default, transport.Encode.Default))

	// compose a dummy task
	me.Task, err = transport.ComposeTask("basic", nil, []interface{}{})
	me.Nil(err)

	// trigger hook
	me.Nil(me.Rpt.ReporterHook(ReporterEvent.BeforeReport, me.Task))

	// send a report
	report, err := me.Task.ComposeReport(transport.Status.Sent, make([]interface{}, 0), nil)
	me.Nil(err)
	{
		b, err := me.Trans.EncodeReport(report)
		me.Nil(err)
		me.Reports <- &ReportEnvelope{
			ID:   report,
			Body: b,
		}
	}

	// polling
	reports, err := me.Sto.Poll(me.Task)
	me.Nil(err)
	me.NotNil(reports)
	select {
	case v, ok := <-reports:
		me.True(ok)
		if !ok {
			break
		}
		r, err := me.Trans.DecodeReport(v)
		me.Nil(err)
		me.True(report.Equal(r))
	}

	// done polling
	me.Nil(me.Sto.Done(me.Task))
}

func (me *BackendTestSuite) TestOrder() {
	// send reports of tasks, make sure their order correct
	me.Nil(me.Trans.Register("order", func() {}, transport.Encode.Default, transport.Encode.Default))

	var (
		tasks []*transport.Task
		wait  sync.WaitGroup
	)

	send := func(task *transport.Task, s int16) {
		r, err := task.ComposeReport(s, nil, nil)
		me.Nil(err)

		b, err := me.Trans.EncodeReport(r)
		me.Nil(err)

		me.Reports <- &ReportEnvelope{task, b}
	}
	chk := func(task *transport.Task, b []byte, s int16) {
		r, err := me.Trans.DecodeReport(b)
		me.Nil(err)

		if r != nil {
			me.Equal(task.ID(), r.ID())
			me.Equal(task.Name(), r.Name())
			me.Equal(s, r.Status())
		}
	}
	gen := func(task *transport.Task, wait *sync.WaitGroup) {
		defer wait.Done()

		me.Nil(me.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

		send(task, transport.Status.Sent)
		send(task, transport.Status.Progress)
		send(task, transport.Status.Success)
	}
	chks := func(task *transport.Task, wait *sync.WaitGroup) {
		defer wait.Done()

		r, err := me.Sto.Poll(task)
		me.Nil(err)

		chk(task, <-r, transport.Status.Sent)
		chk(task, <-r, transport.Status.Progress)
		chk(task, <-r, transport.Status.Success)

		me.Nil(me.Sto.Done(task))
	}

	for i := 0; i < 100; i++ {
		t, err := transport.ComposeTask("order", nil, nil)
		me.Nil(err)
		if t != nil {
			wait.Add(1)
			go gen(t, &wait)

			tasks = append(tasks, t)
		}
	}

	// wait for all routines finished
	wait.Wait()

	for _, v := range tasks {
		wait.Add(1)
		go chks(v, &wait)
	}
	// wait for all chks routine
	wait.Wait()
}
