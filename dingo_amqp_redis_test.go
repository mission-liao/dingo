package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/amqp"
	"github.com/mission-liao/dingo/redis"
	"github.com/stretchr/testify/suite"
)

//
// Amqp(Broker) + Redis(Backend), single App
//

type amqpRedisSingleAppTestSuite struct {
	dingo.DingoSingleAppTestSuite
}

func TestDingoAmqpRedisSingleAppSuite(t *testing.T) {
	suite.Run(t, &amqpRedisSingleAppTestSuite{
		dingo.DingoSingleAppTestSuite{
			GenApp: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(brk, dingo.ObjT.DEFAULT)
				if err != nil {
					return
				}

				bkd, err := dgredis.NewBackend(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(bkd, dingo.ObjT.DEFAULT)
				if err != nil {
					return
				}

				return
			},
		},
	})
}

//
// Amqp(Broker) + Redis(Backend), multi App
//

type amqpRedisMultiAppTestSuite struct {
	dingo.DingoMultiAppTestSuite
}

func TestDingoAmqpRedisMultiAppSuite(t *testing.T) {
	suite.Run(t, &amqpRedisMultiAppTestSuite{
		dingo.DingoMultiAppTestSuite{
			CountOfCallers: 3,
			CountOfWorkers: 3,
			GenCaller: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(brk, dingo.ObjT.PRODUCER)
				if err != nil {
					return
				}
				bkd, err := dgredis.NewBackend(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(bkd, dingo.ObjT.STORE)
				if err != nil {
					return
				}

				return
			},
			GenWorker: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(brk, dingo.ObjT.NAMED_CONSUMER)
				if err != nil {
					return
				}
				bkd, err := dgredis.NewBackend(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(bkd, dingo.ObjT.REPORTER)
				if err != nil {
					return
				}

				return
			},
		},
	})
}
