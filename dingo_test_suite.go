package dingo

import (
	"fmt"
	"time"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type DingoTestSuite struct {
	suite.Suite

	GenBroker  func() (interface{}, error)
	GenBackend func() (Backend, error)
	App_       *App
	Brk        Broker
	Nbrk       NamedBroker
	Bkd        Backend
	eid        int
	events     <-chan *common.Event
}

func (me *DingoTestSuite) SetupSuite() {
	var (
		err error
		ok  bool
	)
	me.App_, err = NewApp("", DefaultConfig())
	me.Nil(err)

	// broker
	v, err := me.GenBroker()
	me.Nil(err)
	me.Brk, ok = v.(Broker)
	if !ok {
		me.Nbrk = v.(NamedBroker)
	}

	// broker
	if me.Brk != nil {
		_, used, err := me.App_.Use(me.Brk, InstT.DEFAULT)
		me.Nil(err)
		me.Equal(InstT.PRODUCER|InstT.CONSUMER, used)
	} else {
		me.NotNil(me.Nbrk)
		_, used, err := me.App_.Use(me.Nbrk, InstT.DEFAULT)
		me.Nil(err)
		me.Equal(InstT.PRODUCER|InstT.CONSUMER, used)
	}

	// backend
	me.Bkd, err = me.GenBackend()
	me.Nil(err)
	_, used, err := me.App_.Use(me.Bkd, InstT.DEFAULT)
	me.Nil(err)
	me.Equal(InstT.REPORTER|InstT.STORE, used)

	// events
	me.eid, me.events, err = me.App_.Listen(InstT.ALL, common.ErrLvl.DEBUG, 0)
	me.Nil(err)
}

func (me *DingoTestSuite) TearDownTest() {
	for {
		done := false
		select {
		case v, ok := <-me.events:
			if !ok {
				done = true
				break
			}
			me.T().Errorf("receiving event:%+v", v)

		// TODO: how to know that there is some error sent...
		// not a good way to block until errors reach here.
		case <-time.After(1 * time.Second):
			done = true
		}

		if done {
			break
		}
	}
}

func (me *DingoTestSuite) TearDownSuite() {
	me.App_.StopListen(me.eid)
	me.Nil(me.App_.Close())
}

func (me *DingoTestSuite) newCaller() (app *App, err error) {
	app, err = NewApp("remote", DefaultConfig())
	me.Nil(err)
	if err != nil {
		return
	}

	v, err := me.GenBroker()
	me.Nil(err)
	if err != nil {
		return
	}

	_, _, err = app.Use(v, InstT.PRODUCER)
	me.Nil(err)
	if err != nil {
		return
	}

	b, err := me.GenBackend()
	me.Nil(err)
	if err != nil {
		return
	}

	_, _, err = app.Use(b, InstT.STORE)
	me.Nil(err)
	if err != nil {
		return
	}

	return
}

func (me *DingoTestSuite) newWorker() (app *App, err error) {
	app, err = NewApp("remote", DefaultConfig())
	me.Nil(err)
	if err != nil {
		return
	}

	v, err := me.GenBroker()
	me.Nil(err)
	if err != nil {
		return
	}

	_, _, err = app.Use(v, InstT.CONSUMER)
	me.Nil(err)
	if err != nil {
		return
	}

	b, err := me.GenBackend()
	me.Nil(err)
	if err != nil {
		return
	}

	_, _, err = app.Use(b, InstT.REPORTER)
	me.Nil(err)
	if err != nil {
		return
	}

	return
}

//
// test cases
//

func (me *DingoTestSuite) TestBasic() {
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

func (me *DingoTestSuite) TestOrder() {
	countOfCallers := 3
	countOfWorkers := 5
	countOfTasks := 5

	{
		_, ok := me.Brk.(*localBroker)
		if ok {
			// localBroker is only valid for 1 caller.
			countOfCallers = 1
		}
	}

	work := func(n int, name string) (int, string) {
		return n + 1, name + "b"
	}

	// init callers
	callers := []*App{}
	defer func() {
		for _, v := range callers {
			me.Nil(v.Close())
		}
	}()
	for i := 0; i < countOfCallers; i++ {
		app, err := me.newCaller()
		me.Nil(err)
		if err != nil {
			return
		}

		// register worker function
		err = app.Register("TestOrder", work, Encode.Default, Encode.Default, ID.Default)
		me.Nil(err)
		if err != nil {
			return
		}

		// set option, make sure 'MonitorProgress' is turned on
		err = app.SetOption("TestOrder", NewOption().SetMonitorProgress(true))
		me.Nil(err)
		if err != nil {
			return
		}

		callers = append(callers, app)
	}

	// init workers
	workers := []*App{}
	defer func() {
		for _, v := range workers {
			me.Nil(v.Close())
		}
	}()
	for i := 0; i < countOfWorkers; i++ {
		app, err := me.newWorker()
		me.Nil(err)
		if err != nil {
			return
		}

		// register worker function
		err = app.Register("TestOrder", work, Encode.Default, Encode.Default, ID.Default)
		me.Nil(err)
		if err != nil {
			return
		}

		// allocate workers
		remain, err := app.Allocate("TestOrder", 1, 1)
		me.Equal(0, remain)
		me.Nil(err)
		if err != nil {
			return
		}

		workers = append(workers, app)
	}

	// sending tasks
	reports := [][]<-chan *transport.Report{}
	for k, v := range callers {
		rs := []<-chan *transport.Report{}
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
