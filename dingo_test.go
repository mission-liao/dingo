package dingo

import (
	"testing"
	"time"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/suite"
)

type DingoTestSuite struct {
	suite.Suite

	app    *_app
	cfg    *Config
	eid    int
	events <-chan *common.Event
}

func (me *DingoTestSuite) SetupSuite() {
	me.cfg = Default()
	app, err := NewApp(*me.cfg)
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
		case <-time.After(2 * time.Second):
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
	_, remain, err := me.app.Register(&StrMatcher{"test_basic"}, func(n int) int {
		called = n
		return n + 1
	}, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// call that function
	reports, err := me.app.Call("test_basic", nil, 5)
	me.Nil(err)
	me.NotNil(reports)

	// await for reports
	status := []int{
		meta.Status.Sent,
		meta.Status.Progress,
		meta.Status.Done,
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
				me.Equal(status[0], v.GetStatus())
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

//
// local(Broker) + local(Backend)
//

type LocalTestSuite struct {
	DingoTestSuite
}

func (me *LocalTestSuite) SetupSuite() {
	me.DingoTestSuite.SetupSuite()

	me.cfg.Broker().Local.Bypass(false)
	me.cfg.Backend().Local.Bypass(false)

	// broker
	{
		v, err := broker.New("local", me.cfg.Broker())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	{
		v, err := backend.New("local", me.cfg.Backend())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)
	}
}

func TestDingoLocalSuite(t *testing.T) {
	suite.Run(t, &LocalTestSuite{})
}

//
// Amqp(Broker) + Amqp(Backend)
//

type AmqpTestSuite struct {
	DingoTestSuite
}

func (me *AmqpTestSuite) SetupSuite() {
	me.DingoTestSuite.SetupSuite()

	// broker
	{
		v, err := broker.New("amqp", me.cfg.Broker())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	{
		v, err := backend.New("amqp", me.cfg.Backend())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)
	}
}

func TestDingoAmqpSuite(t *testing.T) {
	suite.Run(t, &AmqpTestSuite{})
}

//
// Redis(Broker) + Redis(Backend)
//

type RedisTestSuite struct {
	DingoTestSuite
}

func (me *RedisTestSuite) SetupSuite() {
	me.DingoTestSuite.SetupSuite()

	// broker
	{
		v, err := broker.New("redis", me.cfg.Broker())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	{
		v, err := backend.New("redis", me.cfg.Backend())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)
	}
}

func TestDingoRedisSuite(t *testing.T) {
	suite.Run(t, &RedisTestSuite{})
}

//
// Amqp(Broker) + Redis(Backend)
//

type AmqpRedisTestSuite struct {
	DingoTestSuite
}

func (me *AmqpRedisTestSuite) SetupSuite() {
	me.DingoTestSuite.SetupSuite()

	// broker
	{
		v, err := broker.New("amqp", me.cfg.Broker())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	{
		v, err := backend.New("redis", me.cfg.Backend())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)
	}
}

func TestDingoAmqpRedisSuite(t *testing.T) {
	suite.Run(t, &AmqpRedisTestSuite{})
}
