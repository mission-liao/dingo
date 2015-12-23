package dingo

import (
	"fmt"
	"sync"

	"github.com/stretchr/testify/suite"
)

/*
 All dingo.Backend provider should pass this test suite.
 Example testing code:
  type myBackendTestSuite struct {
    dingo.BackendTestSuite
  }
  func TestMyBackendTestSuite(t *testing.T) {
    suite.Run(t, &myBackendTestSuite{
      dingo.BackendTestSuite{
        Gen: func() (dingo.Backend, error) {
          // generate a new instance of your backend.
        },
      },
    })
  }
*/
type BackendTestSuite struct {
	suite.Suite

	Gen     func() (Backend, error)
	Trans   *mgr
	Bkd     Backend
	Rpt     Reporter
	Sto     Store
	Reports chan *ReportEnvelope
	Tasks   []*Task
}

func (me *BackendTestSuite) SetupSuite() {
	me.Trans = newMgr()
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

	me.Tasks = []*Task{}
}

func (me *BackendTestSuite) TearDownTest() {
	me.Nil(me.Bkd.(Object).Close())
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
	me.Nil(me.Trans.Register("basic", func() {}, Encode.Default, Encode.Default, ID.Default))

	// compose a dummy task
	task, err := me.Trans.ComposeTask("basic", nil, []interface{}{})
	me.Nil(err)

	// trigger hook
	me.Nil(me.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

	// send a report
	report, err := task.composeReport(Status.Sent, make([]interface{}, 0), nil)
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
		me.Equal(r, report)
	}

	// done polling
	me.Nil(me.Sto.Done(task))

	me.Tasks = append(me.Tasks, task)
}

func (me *BackendTestSuite) send(task *Task, s int16) {
	r, err := task.composeReport(s, nil, nil)
	me.Nil(err)

	b, err := me.Trans.EncodeReport(r)
	me.Nil(err)

	me.Reports <- &ReportEnvelope{task, b}
}

func (me *BackendTestSuite) chk(task *Task, b []byte, s int16) {
	r, err := me.Trans.DecodeReport(b)
	me.Nil(err)

	if r != nil {
		me.Equal(task.ID(), r.ID())
		me.Equal(task.Name(), r.Name())
		me.Equal(s, r.Status())
	}
}

func (me *BackendTestSuite) gen(task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	me.Nil(me.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

	me.send(task, Status.Sent)
	me.send(task, Status.Progress)
	me.send(task, Status.Success)
}

func (me *BackendTestSuite) chks(task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	r, err := me.Sto.Poll(task)
	me.Nil(err)

	me.chk(task, <-r, Status.Sent)
	me.chk(task, <-r, Status.Progress)
	me.chk(task, <-r, Status.Success)

	me.Nil(me.Sto.Done(task))
}

func (me *BackendTestSuite) TestOrder() {
	// send reports of tasks, make sure their order correct
	me.Nil(me.Trans.Register("order", func() {}, Encode.Default, Encode.Default, ID.Default))

	var (
		tasks []*Task
		wait  sync.WaitGroup
	)

	for i := 0; i < 100; i++ {
		t, err := me.Trans.ComposeTask("order", nil, nil)
		me.Nil(err)
		if t != nil {
			wait.Add(1)
			go me.gen(t, &wait)

			tasks = append(tasks, t)
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

	me.Tasks = append(me.Tasks, tasks...)
}

type testSeqID struct {
	cur int
}

func (me *testSeqID) NewID() string {
	me.cur++
	return fmt.Sprintf("%d", me.cur)
}

func (me *BackendTestSuite) TestSameID() {
	// different type of tasks, with the same id,
	// backend(s) should not get mass.

	var (
		countOfTypes int = 10
		countOfTasks int = 10
		tasks        []*Task
		wait         sync.WaitGroup
	)

	// register idMaker, task
	for i := 0; i < countOfTypes; i++ {
		name := fmt.Sprintf("SameID.%d", i)
		me.Nil(me.Trans.AddIdMaker(100+i, &testSeqID{}))
		me.Nil(me.Trans.Register(name, func() {}, Encode.Default, Encode.Default, 100+i))

		for j := 0; j < countOfTasks; j++ {
			t, err := me.Trans.ComposeTask(name, nil, nil)
			me.Nil(err)
			if t != nil {
				wait.Add(1)
				go me.gen(t, &wait)

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

	me.Tasks = append(me.Tasks, tasks...)
}

func (me *BackendTestSuite) TestExpect() {
	me.NotNil(me.Bkd.(Object).Expect(ObjT.PRODUCER))
	me.NotNil(me.Bkd.(Object).Expect(ObjT.CONSUMER))
	me.NotNil(me.Bkd.(Object).Expect(ObjT.NAMED_CONSUMER))
	me.NotNil(me.Bkd.(Object).Expect(ObjT.ALL))
}
