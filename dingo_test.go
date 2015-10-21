package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/task"
	"github.com/stretchr/testify/suite"
)

type DingoTestSuite struct {
	suite.Suite

	_apps []App
}

func TestDingoSuite(t *testing.T) {
	suite.Run(t, &DingoTestSuite{})
}

func (me *DingoTestSuite) SetupSuite() {
	// local + local
	{
		cfg := Default()
		cfg.Broker().Local_().Bypass(false)
		cfg.Backend().Local_().Bypass(false)
		app, err := NewApp(*cfg)
		me.Nil(err)

		// broker
		{
			v, err := broker.New("local", cfg.Broker())
			me.Nil(err)
			_, used, err := app.Use(v, InstT.DEFAULT)
			me.Nil(err)
			me.Equal(InstT.PRODUCER|InstT.CONSUMER, used)
		}

		// backend
		{
			v, err := backend.New("local", cfg.Backend())
			me.Nil(err)
			_, used, err := app.Use(v, InstT.DEFAULT)
			me.Nil(err)
			me.Equal(InstT.REPORTER|InstT.STORE, used)
		}

		me._apps = append(me._apps, app)
	}

	// TODO: amqp + amqp
	// TODO: amqp + redis
}

func (me *DingoTestSuite) TearDownSuite() {
	for _, v := range me._apps {
		me.Nil(v.Close())
	}
}

//
// test cases
//

func (me *DingoTestSuite) TestBasic() {
	for _, v := range me._apps {
		// register a set of workers
		called := 0
		_, remain, err := v.Register(&StrMatcher{"test_basic"}, func(n int) int {
			called = n
			return n + 1
		}, 1)
		me.Equal(0, remain)
		me.Nil(err)

		// call that function
		reports, err := v.Call("test_basic", nil, 5)
		me.Nil(err)
		me.NotNil(reports)
		if reports == nil {
			continue
		}

		// await for reports
		status := []int{
			task.Status.Sent,
			task.Status.Progress,
			task.Status.Done,
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
					me.Equal(v.GetStatus(), status[0])
					status = status[1:]
				}

				if v.Done() {
					me.Equal(5, called)
					me.Len(v.GetReturn(), 1)
					if len(v.GetReturn()) > 0 {
						ret, ok := v.GetReturn()[0].(int)
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
}
