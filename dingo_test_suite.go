package dingo

import (
	"fmt"

	"github.com/stretchr/testify/suite"
)

/*
 */
type DingoSingleAppTestSuite struct {
	suite.Suite

	GenApp   func() (*App, error)
	App_     *App
	EventMux *mux
}

func (ts *DingoSingleAppTestSuite) SetupSuite()    {}
func (ts *DingoSingleAppTestSuite) TearDownSuite() {}

func (ts *DingoSingleAppTestSuite) SetupTest() {
	var err error
	ts.App_, err = ts.GenApp()
	ts.Nil(err)
	if err != nil {
		return
	}
	_, events, err := ts.App_.Listen(ObjT.All, EventLvl.Debug, 0)
	ts.EventMux = newMux()
	_, err = ts.EventMux.Register(events, 0)
	ts.Nil(err)
	ts.EventMux.Handle(func(val interface{}, _ int) {
		ts.Nil(val)
		ts.Nil(val.(*Event).Payload)
	})
	_, err = ts.EventMux.More(1)
	ts.Nil(err)
}

func (ts *DingoSingleAppTestSuite) TearDownTest() {
	ts.Nil(ts.App_.Close())
	ts.EventMux.Close()
}

//
// test cases
//

func (ts *DingoSingleAppTestSuite) TestBasic() {
	// register a set of workers
	called := 0
	err := ts.App_.Register("TestBasic",
		func(n int) int {
			called = n
			return n + 1
		},
	)
	ts.Nil(err)
	remain, err := ts.App_.Allocate("TestBasic", 1, 1)
	ts.Nil(err)
	ts.Equal(0, remain)

	// call that function
	reports, err := ts.App_.Call("TestBasic", NewOption().SetMonitorProgress(true), 5)
	ts.Nil(err)
	ts.NotNil(reports)

	// await for reports
	status := []int16{
		Status.Sent,
		Status.Progress,
		Status.Success,
	}
	for {
		done := false
		select {
		case v, ok := <-reports:
			ts.True(ok)
			if !ok {
				break
			}

			// make sure the order of status is right
			ts.True(len(status) > 0)
			if len(status) > 0 {
				ts.Equal(status[0], v.Status())
				status = status[1:]
			}

			if v.Done() {
				ts.Equal(5, called)
				ts.Len(v.Return(), 1)
				if len(v.Return()) > 0 {
					ret, ok := v.Return()[0].(int)
					ts.True(ok)
					ts.Equal(called+1, ret)
				}
				done = true
			}
		}

		if done {
			break
		}
	}
}

/*
 */
type DingoMultiAppTestSuite struct {
	suite.Suite

	CountOfCallers, CountOfWorkers int
	GenCaller                      func() (*App, error)
	GenWorker                      func() (*App, error)
	Callers, Workers               []*App
	EventMux                       *mux
}

func (ts *DingoMultiAppTestSuite) SetupSuite() {
	ts.NotEqual(0, ts.CountOfCallers)
	ts.NotEqual(0, ts.CountOfWorkers)
}
func (ts *DingoMultiAppTestSuite) TearDownSuite() {}

func (ts *DingoMultiAppTestSuite) SetupTest() {
	var err error
	ts.EventMux = newMux()
	ts.EventMux.Handle(func(val interface{}, _ int) {
		ts.Nil(val)
	})
	_, err = ts.EventMux.More(1)
	ts.Nil(err)

	// prepare callers
	for i := 0; i < ts.CountOfCallers; i++ {
		app, err := ts.GenCaller()
		ts.Nil(err)
		if err != nil {
			return
		}
		ts.Callers = append(ts.Callers, app)

		// listen to events
		_, events, err := app.Listen(ObjT.All, EventLvl.Debug, 0)
		ts.Nil(err)
		if err != nil {
			return
		}
		_, err = ts.EventMux.Register(events, 0)
		ts.Nil(err)
		if err != nil {
			return
		}
	}

	// prepare workers
	for i := 0; i < ts.CountOfWorkers; i++ {
		app, err := ts.GenWorker()
		ts.Nil(err)
		if err != nil {
			return
		}
		ts.Workers = append(ts.Workers, app)
	}
}

func (ts *DingoMultiAppTestSuite) TearDownTest() {
	for _, v := range ts.Callers {
		ts.Nil(v.Close())
	}

	for _, v := range ts.Workers {
		ts.Nil(v.Close())
	}

	ts.EventMux.Close()
}

func (ts *DingoMultiAppTestSuite) register(name string, fn interface{}) {
	for _, v := range ts.Callers {
		ts.Nil(v.Register(name, fn))
	}
	for _, v := range ts.Workers {
		ts.Nil(v.Register(name, fn))
	}
}

func (ts *DingoMultiAppTestSuite) setOption(name string, opt *Option) {
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
	ts.setOption("TestOrder", NewOption().SetMonitorProgress(true))
	ts.allocate("TestOrder", 1, 1)

	// sending tasks
	reports := [][]<-chan *Report{}
	for k, v := range ts.Callers {
		rs := []<-chan *Report{}
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
			ts.Equal(Status.Sent, rep.Status())
			name, id := rep.Name(), rep.ID()

			// progress
			rep = <-v
			ts.Equal(Status.Progress, rep.Status())
			ts.Equal(name, rep.Name())
			ts.Equal(id, rep.ID())

			// success
			rep = <-v
			ts.Equal(Status.Success, rep.Status())
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
