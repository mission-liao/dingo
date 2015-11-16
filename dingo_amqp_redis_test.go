package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/stretchr/testify/suite"
)

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

	me.Nil(me.app.Init(*me.cfg))
}

func TestDingoAmqpRedisSuite(t *testing.T) {
	suite.Run(t, &AmqpRedisTestSuite{})
}
