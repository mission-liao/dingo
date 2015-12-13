package tests

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/amqp"
	"github.com/mission-liao/dingo/redis"
	"github.com/stretchr/testify/suite"
)

//
// Amqp(Broker) + Redis(Backend)
//

type amqpRedisTestSuite struct {
	dingo.DingoTestSuite
}

func (me *amqpRedisTestSuite) SetupSuite() {
	var err error
	// broker
	me.Nbrk, err = dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
	me.Nil(err)

	// backend
	me.Bkd, err = dgredis.NewBackend(dgredis.DefaultRedisConfig())
	me.Nil(err)

	me.DingoTestSuite.SetupSuite()
}

func TestDingoAmqpRedisSuite(t *testing.T) {
	suite.Run(t, &amqpRedisTestSuite{})
}
