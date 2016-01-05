package dingo_test

import (
	"fmt"
	"sync"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

/*
 */
type DingoSingleAppTestSuite struct {
	suite.Suite

	GenApp        func() (*dingo.App, error)
	app           *dingo.App
	eventRoutines *dingo.Routines
}

func (ts *DingoSingleAppTestSuite) SetupSuite() {
	ts.eventRoutines = dingo.NewRoutines()
}
func (ts *DingoSingleAppTestSuite) TearDownSuite() {}
func (ts *DingoSingleAppTestSuite) SetupTest() {
	var err error
	defer func() {
		ts.Nil(err)
	}()

	ts.app, err = ts.GenApp()
	if err != nil {
		return
	}

	_, events, err := ts.app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
	if err != nil {
		return
	}

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

func (ts *DingoSingleAppTestSuite) TearDownTest() {
	ts.Nil(ts.app.Close())
	ts.eventRoutines.Close()
}

//
// test cases
//

func (ts *DingoSingleAppTestSuite) TestBasic() {
	// register a set of workers
	called := 0
	err := ts.app.Register("TestBasic",
		func(n int) int {
			called = n
			return n + 1
		},
	)
	ts.Nil(err)
	remain, err := ts.app.Allocate("TestBasic", 1, 1)
	ts.Nil(err)
	ts.Equal(0, remain)

	// call that function
	reports, err := ts.app.Call("TestBasic", dingo.DefaultOption().MonitorProgress(true), 5)
	ts.Nil(err)
	ts.NotNil(reports)

	// await for reports
	status := []int16{
		dingo.Status.Sent,
		dingo.Status.Progress,
		dingo.Status.Success,
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

func (ts *DingoSingleAppTestSuite) TestSameID() {
	// tasks of different name have the same IDs
	var (
		err                error
		countOfConcurrency = 12
		countOfTasks       = 5
		results            = make([][]*dingo.Result, countOfConcurrency)
		wait               sync.WaitGroup
	)
	defer func() {
		ts.Nil(err)
	}()
	fn := func(n int, msg string) (int, string) {
		return n + 1000, "the msg: " + msg
	}

	for i := 0; i < countOfConcurrency; i++ {
		name := fmt.Sprintf("TestSameID.%d", i)
		err = ts.app.Register(name, fn)
		if err != nil {
			return
		}
		err = ts.app.AddIDMaker(100+i, &dingo.SeqIDMaker{})
		if err != nil {
			return
		}
		err = ts.app.SetIDMaker(name, 100+i)
		if err != nil {
			return
		}
		err = ts.app.SetOption(name, dingo.DefaultOption().MonitorProgress(true))
		if err != nil {
			return
		}
		_, err = ts.app.Allocate(name, 5, 3)
		if err != nil {
			return
		}
	}

	// send tasks in parellel
	wait.Add(countOfConcurrency)
	{
		for i := 0; i < countOfConcurrency; i++ {
			go func(idx int, name string) {
				defer wait.Done()
				for j := 0; j < countOfTasks; j++ {
					results[idx] = append(
						results[idx],
						dingo.NewResult(
							ts.app.Call(
								name, nil,
								idx*countOfConcurrency+j, fmt.Sprintf("%d", j),
							)))
				}
			}(i, fmt.Sprintf("TestSameID.%d", i))
		}
	}
	wait.Wait()

	// receiving reports
	received := 0
	defer func() {
		ts.Equal(countOfConcurrency*countOfTasks, received)
	}()
	for i, v := range results {
		for j, r := range v {
			err = r.Wait(0)
			if err != nil {
				return
			}

			ts.Equal(i*countOfConcurrency+j+1000, r.Last.Return()[0].(int))
			ts.Equal(fmt.Sprintf("the msg: %d", j), r.Last.Return()[1].(string))
			received++
		}
	}
}
