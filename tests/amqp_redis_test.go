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

func TestDingoAmqpRedisSuite(t *testing.T) {
	suite.Run(t, &amqpRedisTestSuite{
		dingo.DingoTestSuite{
			GenBroker: func() (v interface{}, err error) {
				v, err = dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
				return
			},
			GenBackend: func() (b dingo.Backend, err error) {
				b, err = dgredis.NewBackend(dgredis.DefaultRedisConfig())
				return
			},
		},
	})
}
