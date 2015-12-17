package dingo

import (
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
		_, used, err := me.App_.Use(me.Brk, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	} else {
		me.NotNil(me.Nbrk)
		_, used, err := me.App_.Use(me.Nbrk, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	me.Bkd, err = me.GenBackend()
	me.Nil(err)
	_, used, err := me.App_.Use(me.Bkd, common.InstT.DEFAULT)
	me.Nil(err)
	me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)

	// events
	me.eid, me.events, err = me.App_.Listen(common.InstT.ALL, common.ErrLvl.DEBUG, 0)
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
		}, transport.Encode.Default, transport.Encode.Default,
	)
	me.Nil(err)
	remain, err := me.App_.Allocate("TestBasic", 1, 1)
	me.Nil(err)
	me.Equal(0, remain)

	// call that function
	reports, err := me.App_.Call("TestBasic", transport.NewOption().SetMonitorProgress(true), 5)
	me.Nil(err)
	me.NotNil(reports)

	// await for reports
	status := []int16{
		transport.Status.Sent,
		transport.Status.Progress,
		transport.Status.Success,
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
