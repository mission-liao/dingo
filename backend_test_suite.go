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
	Trans   *fnMgr
	Bkd     Backend
	Rpt     Reporter
	Sto     Store
	Reports chan *ReportEnvelope
	Tasks   []*Task
}

func (ts *BackendTestSuite) SetupSuite() {
	ts.Trans = newFnMgr()
	ts.NotNil(ts.Gen)
}

func (ts *BackendTestSuite) TearDownSuite() {
}

func (ts *BackendTestSuite) SetupTest() {
	var err error

	ts.Bkd, err = ts.Gen()
	ts.Nil(err)
	ts.Rpt, ts.Sto = ts.Bkd.(Reporter), ts.Bkd.(Store)
	ts.NotNil(ts.Rpt)
	ts.NotNil(ts.Sto)

	ts.Reports = make(chan *ReportEnvelope, 10)
	_, err = ts.Rpt.Report(ts.Reports)
	ts.Nil(err)

	ts.Tasks = []*Task{}
}

func (ts *BackendTestSuite) TearDownTest() {
	ts.Nil(ts.Bkd.(Object).Close())
	ts.Bkd, ts.Rpt, ts.Sto = nil, nil, nil

	close(ts.Reports)
	ts.Reports = nil

	ts.Tasks = nil
}

//
// test cases
//

func (ts *BackendTestSuite) TestBasic() {
	// register an encoding for this method
	ts.Nil(ts.Trans.Register("basic", func() {}))

	// compose a dummy task
	task, err := ts.Trans.ComposeTask("basic", nil, []interface{}{})
	ts.Nil(err)

	// trigger hook
	ts.Nil(ts.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

	// send a report
	report, err := task.composeReport(Status.Sent, make([]interface{}, 0), nil)
	ts.Nil(err)
	{
		b, err := ts.Trans.EncodeReport(report)
		ts.Nil(err)
		ts.Reports <- &ReportEnvelope{
			ID:   report,
			Body: b,
		}
	}

	// polling
	reports, err := ts.Sto.Poll(task)
	ts.Nil(err)
	ts.NotNil(reports)
	select {
	case v, ok := <-reports:
		ts.True(ok)
		if !ok {
			break
		}
		r, err := ts.Trans.DecodeReport(v)
		ts.Nil(err)
		ts.Equal(r, report)
	}

	// done polling
	ts.Nil(ts.Sto.Done(task))

	ts.Tasks = append(ts.Tasks, task)
}

func (ts *BackendTestSuite) send(task *Task, s int16) {
	r, err := task.composeReport(s, nil, nil)
	ts.Nil(err)

	b, err := ts.Trans.EncodeReport(r)
	ts.Nil(err)

	ts.Reports <- &ReportEnvelope{task, b}
}

func (ts *BackendTestSuite) chk(task *Task, b []byte, s int16) {
	r, err := ts.Trans.DecodeReport(b)
	ts.Nil(err)

	if r != nil {
		ts.Equal(task.ID(), r.ID())
		ts.Equal(task.Name(), r.Name())
		ts.Equal(s, r.Status())
	}
}

func (ts *BackendTestSuite) gen(task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	ts.Nil(ts.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

	ts.send(task, Status.Sent)
	ts.send(task, Status.Progress)
	ts.send(task, Status.Success)
}

func (ts *BackendTestSuite) chks(task *Task, wait *sync.WaitGroup) {
	defer wait.Done()

	r, err := ts.Sto.Poll(task)
	ts.Nil(err)

	ts.chk(task, <-r, Status.Sent)
	ts.chk(task, <-r, Status.Progress)
	ts.chk(task, <-r, Status.Success)

	ts.Nil(ts.Sto.Done(task))
}

func (ts *BackendTestSuite) TestOrder() {
	// send reports of tasks, make sure their order correct
	ts.Nil(ts.Trans.Register("order", func() {}))

	var (
		tasks []*Task
		wait  sync.WaitGroup
	)

	for i := 0; i < 100; i++ {
		t, err := ts.Trans.ComposeTask("order", nil, nil)
		ts.Nil(err)
		if t != nil {
			wait.Add(1)
			go ts.gen(t, &wait)

			tasks = append(tasks, t)
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

	ts.Tasks = append(ts.Tasks, tasks...)
}

type testSeqID struct {
	cur int
}

func (ts *testSeqID) NewID() (string, error) {
	ts.cur++
	return fmt.Sprintf("%d", ts.cur), nil
}

func (ts *BackendTestSuite) TestSameID() {
	// different type of tasks, with the same id,
	// backend(s) should not get mass.

	var (
		countOfTypes = 10
		countOfTasks = 10
		tasks        []*Task
		wait         sync.WaitGroup
	)

	// register idMaker, task
	for i := 0; i < countOfTypes; i++ {
		name := fmt.Sprintf("SameID.%d", i)
		ts.Nil(ts.Trans.AddIDMaker(100+i, &testSeqID{}))
		ts.Nil(ts.Trans.Register(name, func() {}))
		ts.Nil(ts.Trans.SetIDMaker(name, 100+i))

		for j := 0; j < countOfTasks; j++ {
			t, err := ts.Trans.ComposeTask(name, nil, nil)
			ts.Nil(err)
			if t != nil {
				wait.Add(1)
				go ts.gen(t, &wait)

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

	ts.Tasks = append(ts.Tasks, tasks...)
}

func (ts *BackendTestSuite) TestExpect() {
	ts.NotNil(ts.Bkd.(Object).Expect(ObjT.Producer))
	ts.NotNil(ts.Bkd.(Object).Expect(ObjT.Consumer))
	ts.NotNil(ts.Bkd.(Object).Expect(ObjT.NamedConsumer))
	ts.NotNil(ts.Bkd.(Object).Expect(ObjT.All))
}
