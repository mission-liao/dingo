package dingo

import (
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BackendTestSuite struct {
	suite.Suite

	Gen     func() (Backend, error)
	Trans   *transport.Mgr
	Bkd     Backend
	Rpt     Reporter
	Sto     Store
	Reports chan *ReportEnvelope
	Tasks   []*transport.Task
}

func (me *BackendTestSuite) SetupSuite() {
	me.Trans = transport.NewMgr()
	me.NotNil(me.Gen)
}

func (me *BackendTestSuite) TearDownSuite() {
}

func (me *BackendTestSuite) SetupTest() {
	var err error

	me.Bkd, err = me.Gen()
	me.Nil(err)
	me.Rpt, me.Sto = me.Bkd.(Reporter), me.Bkd.(Store)
	me.NotNil(me.Rpt)
	me.NotNil(me.Sto)

	me.Reports = make(chan *ReportEnvelope, 10)
	_, err = me.Rpt.Report(me.Reports)
	me.Nil(err)

	me.Tasks = []*transport.Task{}
}

func (me *BackendTestSuite) TearDownTest() {
	me.Nil(me.Bkd.(common.Object).Close())
	me.Bkd, me.Rpt, me.Sto = nil, nil, nil

	close(me.Reports)
	me.Reports = nil

	me.Tasks = nil
}

//
// test cases
//

func (me *BackendTestSuite) TestBasic() {
	// register an encoding for this method
	me.Nil(me.Trans.Register("basic", func() {}, transport.Encode.Default, transport.Encode.Default))

	// compose a dummy task
	task, err := transport.ComposeTask("basic", nil, []interface{}{})
	me.Nil(err)

	// trigger hook
	me.Nil(me.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

	// send a report
	report, err := task.ComposeReport(transport.Status.Sent, make([]interface{}, 0), nil)
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
	reports, err := me.Sto.Poll(task)
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
	me.Nil(me.Sto.Done(task))

	me.Tasks = append(me.Tasks, task)
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

	me.Tasks = append(me.Tasks, tasks...)
}
