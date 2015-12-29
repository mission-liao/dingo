package dingo_test

import (
	"fmt"
	"sync"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

/*
 */
type DingoMultiAppTestSuite struct {
	suite.Suite

	CountOfCallers, CountOfWorkers int
	GenCaller                      func() (*dingo.App, error)
	GenWorker                      func() (*dingo.App, error)
	Callers, Workers               []*dingo.App
	eventRoutines                  *dingo.Routines
}

func (ts *DingoMultiAppTestSuite) SetupSuite() {
	ts.NotEqual(0, ts.CountOfCallers)
	ts.NotEqual(0, ts.CountOfWorkers)
	ts.eventRoutines = dingo.NewRoutines()
}
func (ts *DingoMultiAppTestSuite) TearDownSuite() {}

func (ts *DingoMultiAppTestSuite) SetupTest() {
	var err error
	defer func() {
		ts.Nil(err)
	}()

	// prepare callers
	for i := 0; i < ts.CountOfCallers; i++ {
		app, err := ts.GenCaller()
		if err != nil {
			return
		}
		ts.Callers = append(ts.Callers, app)

		// listen to events
		_, events, err := app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
		if err != nil {
			return
		}
		ts.listenTo(events)
	}

	// prepare workers
	for i := 0; i < ts.CountOfWorkers; i++ {
		app, err := ts.GenWorker()
		if err != nil {
			return
		}
		ts.Workers = append(ts.Workers, app)

		// listen to events
		_, events, err := app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
		if err != nil {
			return
		}
		ts.listenTo(events)
	}
}

func (ts *DingoMultiAppTestSuite) TearDownTest() {
	for _, v := range ts.Callers {
		ts.Nil(v.Close())
	}

	for _, v := range ts.Workers {
		ts.Nil(v.Close())
	}

	ts.eventRoutines.Close()
	ts.Callers, ts.Workers = []*dingo.App{}, []*dingo.App{}
}

func (ts *DingoMultiAppTestSuite) listenTo(events <-chan *dingo.Event) {
	go func(quit <-chan int, wait *sync.WaitGroup, events <-chan *dingo.Event) {
		defer wait.Done()
		chk := func(e *dingo.Event) {
			if e.Level >= dingo.EventLvl.Warning {
				ts.Nil(e)
			}
		}
		for {
			select {
			case _, _ = <-quit:
				goto clean
			case e, ok := <-events:
				if !ok {
					goto clean
				}
				chk(e)
			}
		}
	clean:
		for {
			select {
			default:
				break clean
			case e, ok := <-events:
				if !ok {
					break clean
				}
				chk(e)
			}
		}
	}(ts.eventRoutines.New(), ts.eventRoutines.Wait(), events)
}

func (ts *DingoMultiAppTestSuite) register(name string, fn interface{}) {
	for _, v := range ts.Callers {
		ts.Nil(v.Register(name, fn))
	}
	for _, v := range ts.Workers {
		ts.Nil(v.Register(name, fn))
	}
}

func (ts *DingoMultiAppTestSuite) setOption(name string, opt *dingo.Option) {
	for _, v := range ts.Callers {
		ts.Nil(v.SetOption(name, opt))
	}
}

func (ts *DingoMultiAppTestSuite) allocate(name string, count, share int) {
	for _, v := range ts.Workers {
		remain, err := v.Allocate(name, count, share)
		ts.Equal(0, remain)
		ts.Nil(err)
	}
}

//
// test cases
//

func (ts *DingoMultiAppTestSuite) TestOrder() {
	countOfTasks := 5
	work := func(n int, name string) (int, string) {
		return n + 1, name + "b"
	}

	// register worker function
	ts.register("TestOrder", work)
	ts.setOption("TestOrder", dingo.DefaultOption().MonitorProgress(true))
	ts.allocate("TestOrder", 1, 1)

	// sending tasks
	reports := [][]<-chan *dingo.Report{}
	for k, v := range ts.Callers {
		rs := []<-chan *dingo.Report{}
		for i := 0; i < countOfTasks; i++ {
			rep, err := v.Call("TestOrder", nil, k, fmt.Sprintf("%d.%d", k, i))
			ts.Nil(err)
			if err != nil {
				return
			}

			rs = append(rs, rep)
		}
		reports = append(reports, rs)
	}

	// check results one by one
	for k, rs := range reports {
		for i, v := range rs {
			// sent
			rep := <-v
			ts.Equal(dingo.Status.Sent, rep.Status())
			name, id := rep.Name(), rep.ID()

			// progress
			rep = <-v
			ts.Equal(dingo.Status.Progress, rep.Status())
			ts.Equal(name, rep.Name())
			ts.Equal(id, rep.ID())

			// success
			rep = <-v
			ts.Equal(dingo.Status.Success, rep.Status())
			ts.Equal(name, rep.Name())
			ts.Equal(id, rep.ID())

			// check result
			ret := rep.Return()
			ts.Len(ret, 2)
			if len(ret) == 2 {
				// plus 1
				ts.Equal(k+1, ret[0].(int))
				// plus 'b'
				ts.Equal(fmt.Sprintf("%d.%db", k, i), ret[1].(string))
			}
		}
	}
}

func (ts *DingoMultiAppTestSuite) TestSameID() {
	var (
		err          error
		countOfTasks = 5
		reports      = make([][]*dingo.Result, 0, len(ts.Callers))
		fn           = func(n int, msg string) (int, string) {
			return n + 1000, "the msg: " + msg
		}
	)
	defer func() {
		ts.Nil(err)
	}()

	// prepare caller
	for k, v := range ts.Callers {
		name := fmt.Sprintf("TestSameID.%v", k)

		ts.register(name, fn)
		ts.allocate(name, 2, 2)

		err = v.AddIDMaker(101, &testIDMaker{})
		if err != nil {
			return
		}
		err = v.SetIDMaker(name, 101)
		if err != nil {
			return
		}

		rs := []*dingo.Result{}
		for i := 0; i < countOfTasks; i++ {
			rs = append(rs, dingo.NewResult(
				v.Call(
					name, nil,
					k*100+i, fmt.Sprintf("%d", k*100+i),
				)))
		}
		reports = append(reports, rs)
	}

	// checking result
	received := 0
	defer func() {
		ts.Equal(countOfTasks*len(ts.Callers), received)
	}()
	for k, rs := range reports {
		for i, r := range rs {
			err = r.Wait(0)
			if err != nil {
				return
			}
			ts.True(r.Last.OK())
			ts.Equal(1000+k*100+i, r.Last.Return()[0].(int))
			ts.Equal(fmt.Sprintf("the msg: %d", k*100+i), r.Last.Return()[1].(string))
			received++
		}
	}
}
