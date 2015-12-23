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

func (me *DingoSingleAppTestSuite) SetupSuite()    {}
func (me *DingoSingleAppTestSuite) TearDownSuite() {}

func (me *DingoSingleAppTestSuite) SetupTest() {
	var err error
	me.App_, err = me.GenApp()
	me.Nil(err)
	if err != nil {
		return
	}
	_, events, err := me.App_.Listen(ObjT.ALL, EventLvl.DEBUG, 0)
	me.EventMux = newMux()
	_, err = me.EventMux.Register(events, 0)
	me.Nil(err)
	me.EventMux.Handle(func(val interface{}, _ int) {
		me.Nil(val)
		me.Nil(val.(*Event).Payload)
	})
	_, err = me.EventMux.More(1)
	me.Nil(err)
}

func (me *DingoSingleAppTestSuite) TearDownTest() {
	me.Nil(me.App_.Close())
	me.EventMux.Close()
}

//
// test cases
//

func (me *DingoSingleAppTestSuite) TestBasic() {
	// register a set of workers
	called := 0
	err := me.App_.Register("TestBasic",
		func(n int) int {
			called = n
			return n + 1
		}, Encode.Default, Encode.Default, ID.Default,
	)
	me.Nil(err)
	remain, err := me.App_.Allocate("TestBasic", 1, 1)
	me.Nil(err)
	me.Equal(0, remain)

	// call that function
	reports, err := me.App_.Call("TestBasic", NewOption().SetMonitorProgress(true), 5)
	me.Nil(err)
	me.NotNil(reports)

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
			me.True(ok)
			if !ok {
				break
			}

			// make sure the order of status is right
			me.True(len(status) > 0)
			if len(status) > 0 {
				me.Equal(status[0], v.Status())
				status = status[1:]
			}

			if v.Done() {
				me.Equal(5, called)
				me.Len(v.Return(), 1)
				if len(v.Return()) > 0 {
					ret, ok := v.Return()[0].(int)
					me.True(ok)
					me.Equal(called+1, ret)
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

func (me *DingoMultiAppTestSuite) SetupSuite()    {}
func (me *DingoMultiAppTestSuite) TearDownSuite() {}

func (me *DingoMultiAppTestSuite) SetupTest() {
	var err error
	me.EventMux = newMux()
	me.EventMux.Handle(func(val interface{}, _ int) {
		me.Nil(val)
	})
	_, err = me.EventMux.More(1)
	me.Nil(err)

	// prepare callers
	for i := 0; i < me.CountOfCallers; i++ {
		app, err := me.GenCaller()
		me.Nil(err)
		if err != nil {
			return
		}
		me.Callers = append(me.Callers, app)

		// listen to events
		_, events, err := app.Listen(ObjT.ALL, EventLvl.DEBUG, 0)
		me.Nil(err)
		if err != nil {
			return
		}
		_, err = me.EventMux.Register(events, 0)
		me.Nil(err)
		if err != nil {
			return
		}
	}

	// prepare workers
	for i := 0; i < me.CountOfWorkers; i++ {
		app, err := me.GenWorker()
		me.Nil(err)
		if err != nil {
			return
		}
		me.Workers = append(me.Workers, app)
	}
}

func (me *DingoMultiAppTestSuite) TearDownTest() {
	for _, v := range me.Callers {
		me.Nil(v.Close())
	}

	for _, v := range me.Workers {
		me.Nil(v.Close())
	}

	me.EventMux.Close()
}

func (me *DingoMultiAppTestSuite) register(name string, fn interface{}, encodeT, encodeR, id int) {
	for _, v := range me.Callers {
		me.Nil(v.Register(name, fn, encodeT, encodeR, id))
	}
	for _, v := range me.Workers {
		me.Nil(v.Register(name, fn, encodeT, encodeR, id))
	}
}

func (me *DingoMultiAppTestSuite) setOption(name string, opt *Option) {
	for _, v := range me.Callers {
		me.Nil(v.SetOption(name, opt))
	}
}

func (me *DingoMultiAppTestSuite) allocate(name string, count, share int) {
	for _, v := range me.Workers {
		remain, err := v.Allocate(name, count, share)
		me.Equal(0, remain)
		me.Nil(err)
	}
}

//
// test cases
//

func (me *DingoMultiAppTestSuite) TestOrder() {
	countOfTasks := 5
	work := func(n int, name string) (int, string) {
		return n + 1, name + "b"
	}

	// register worker function
	me.register("TestOrder", work, Encode.Default, Encode.Default, ID.Default)
	me.setOption("TestOrder", NewOption().SetMonitorProgress(true))
	me.allocate("TestOrder", 1, 1)

	// sending tasks
	reports := [][]<-chan *Report{}
	for k, v := range me.Callers {
		rs := []<-chan *Report{}
		for i := 0; i < countOfTasks; i++ {
			rep, err := v.Call("TestOrder", nil, k, fmt.Sprintf("%d.%d", k, i))
			me.Nil(err)
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
			me.Equal(Status.Sent, rep.Status())
			name, id := rep.Name(), rep.ID()

			// progress
			rep = <-v
			me.Equal(Status.Progress, rep.Status())
			me.Equal(name, rep.Name())
			me.Equal(id, rep.ID())

			// success
			rep = <-v
			me.Equal(Status.Success, rep.Status())
			me.Equal(name, rep.Name())
			me.Equal(id, rep.ID())

			// check result
			ret := rep.Return()
			me.Len(ret, 2)
			if len(ret) == 2 {
				// plus 1
				me.Equal(k+1, ret[0].(int))
				// plus 'b'
				me.Equal(fmt.Sprintf("%d.%db", k, i), ret[1].(string))
			}
		}
	}
}
