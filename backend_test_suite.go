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

	Gen   func() (Backend, error)
	Trans *fnMgr
	Bkd   Backend
	Rpt   Reporter
	Sto   Store
	Tasks []*Task
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
	ts.Tasks = []*Task{}
}

func (ts *BackendTestSuite) TearDownTest() {
	ts.Nil(ts.Bkd.(Object).Close())
	ts.Bkd, ts.Rpt, ts.Sto = nil, nil, nil

	ts.Tasks = nil
}

//
// test cases
//

func (ts *BackendTestSuite) TestBasic() {
	var err error
	defer func() {
		ts.Nil(err)
	}()

	// register an encoding for this method
	err = ts.Trans.Register("basic", func() {})
	if err != nil {
		return
	}

	// compose a dummy task
	task, err := ts.Trans.ComposeTask("basic", nil, []interface{}{})
	if err != nil {
		return
	}

	// trigger hook
	err = ts.Rpt.ReporterHook(ReporterEvent.BeforeReport, task)
	if err != nil {
		return
	}

	// register a report channel
	reports := make(chan *ReportEnvelope, 10)
	_, err = ts.Rpt.Report("basic", reports)
	if err != nil {
		return
	}

	// send a report
	report, err := task.composeReport(Status.Sent, make([]interface{}, 0), nil)
	if err != nil {
		return
	}
	b, err := ts.Trans.EncodeReport(report)
	if err != nil {
		return
	}
	reports <- &ReportEnvelope{
		ID:   report,
		Body: b,
	}

	// polling
	rs, err := ts.Sto.Poll(task)
	if err != nil {
		return
	}
	ts.NotNil(rs)
	select {
	case v, ok := <-rs:
		ts.True(ok)
		if !ok {
			break
		}
		r, err := ts.Trans.DecodeReport(v)
		ts.Nil(err)
		ts.Equal(r, report)
	}

	// done polling
	err = ts.Sto.Done(task)
	if err != nil {
		return
	}

	ts.Tasks = append(ts.Tasks, task)
}

func (ts *BackendTestSuite) send(task *Task, out chan<- *ReportEnvelope, s int16) {
	r, err := task.composeReport(s, nil, nil)
	ts.Nil(err)

	b, err := ts.Trans.EncodeReport(r)
	ts.Nil(err)

	out <- &ReportEnvelope{task, b}
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

func (ts *BackendTestSuite) gen(task *Task, out chan<- *ReportEnvelope, wait *sync.WaitGroup) {
	defer wait.Done()

	ts.Nil(ts.Rpt.ReporterHook(ReporterEvent.BeforeReport, task))

	ts.send(task, out, Status.Sent)
	ts.send(task, out, Status.Progress)
	ts.send(task, out, Status.Success)
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
	var (
		err     error
		tasks   []*Task
		t       *Task
		wait    sync.WaitGroup
		reports = make(chan *ReportEnvelope, 10)
	)
	defer func() {
		ts.Nil(err)
	}()

	// send reports of tasks, make sure their order correct
	err = ts.Trans.Register("order", func() {})
	if err != nil {
		return
	}

	_, err = ts.Rpt.Report("order", reports)
	if err != nil {
		return
	}

	for i := 0; i < 100; i++ {
		t, err = ts.Trans.ComposeTask("order", nil, nil)
		if err != nil {
			return
		}
		if t != nil {
			wait.Add(1)
			go ts.gen(t, reports, &wait)

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
		t            *Task
		wait         sync.WaitGroup
		err          error
	)
	defer func() {
		ts.Nil(err)
	}()

	// register idMaker, task
	for i := 0; i < countOfTypes; i++ {
		name := fmt.Sprintf("SameID.%d", i)
		err = ts.Trans.AddIDMaker(100+i, &testSeqID{})
		if err != nil {
			return
		}
		err = ts.Trans.Register(name, func() {})
		if err != nil {
			return
		}
		err = ts.Trans.SetIDMaker(name, 100+i)
		if err != nil {
			return
		}

		reports := make(chan *ReportEnvelope, 10)
		_, err = ts.Rpt.Report(name, reports)
		if err != nil {
			return
		}

		for j := 0; j < countOfTasks; j++ {
			t, err = ts.Trans.ComposeTask(name, nil, nil)
			if err != nil {
				return
			}
			if t != nil {
				wait.Add(1)
				go ts.gen(t, reports, &wait)

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
