package tests

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/redis"
	"github.com/stretchr/testify/suite"
)

//
// Redis(Broker) + Redis(Backend)
//

type redisTestSuite struct {
	dingo.DingoTestSuite
}

func (me *redisTestSuite) SetupSuite() {
	var err error
	// broker
	me.Nbrk, err = dgredis.NewBroker(dgredis.DefaultRedisConfig())
	me.Nil(err)

	// backend
	me.Bkd, err = dgredis.NewBackend(dgredis.DefaultRedisConfig())
	me.Nil(err)

	me.DingoTestSuite.SetupSuite()
}

func TestDingoRedisSuite(t *testing.T) {
	suite.Run(t, &redisTestSuite{})
}
