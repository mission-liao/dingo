package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/stretchr/testify/suite"
)

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

	me.Nil(me.app.Init(*me.cfg))
}

func TestDingoAmqpSuite(t *testing.T) {
	suite.Run(t, &AmqpTestSuite{})
}
