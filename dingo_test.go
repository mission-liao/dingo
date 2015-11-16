package dingo

import (
	"time"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

// TODO: add test case for return-fix

type DingoTestSuite struct {
	suite.Suite

	cfg    *Config
	app    *_app
	eid    int
	events <-chan *common.Event
}

func (me *DingoTestSuite) SetupSuite() {
	me.cfg = Default()
	app, err := NewApp("")
	me.Nil(err)
	me.app = app.(*_app)
	me.eid, me.events, err = me.app.Listen(common.InstT.ALL, common.ErrLvl.DEBUG, 0)
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
	me.app.StopListen(me.eid)
	me.Nil(me.app.Close())
}

//
// test cases
//

func (me *DingoTestSuite) TestBasic() {
	// register a set of workers
	called := 0
	remain, err := me.app.Register("Basic", func(n int) int {
		called = n
		return n + 1
	}, 1, 1, transport.Encode.Default, transport.Encode.Default)
	me.Equal(0, remain)
	me.Nil(err)

	// call that function
	reports, err := me.app.Call("Basic", nil, 5)
	me.Nil(err)
	me.NotNil(reports)

	// await for reports
	status := []int{
		transport.Status.Sent,
		transport.Status.Progress,
		transport.Status.Done,
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
