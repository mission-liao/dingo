package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/redis"
	"github.com/stretchr/testify/suite"
)

//
// Redis(Broker) + Redis(Backend), Single App
//

type redisSingleAppTestSuite struct {
	dingo.DingoSingleAppTestSuite
}

func TestDingoRedisSingleAppSuite(t *testing.T) {
	suite.Run(t, &redisSingleAppTestSuite{
		dingo.DingoSingleAppTestSuite{
			GenApp: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgredis.NewBroker(dgredis.DefaultRedisConfig())
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
// Redis(Broker) + Redis(Backend), Multi App
//

type redisMultiAppTestSuite struct {
	dingo.DingoMultiAppTestSuite
}

func TestDingoRedisMultiAppSuite(t *testing.T) {
	suite.Run(t, &redisMultiAppTestSuite{
		dingo.DingoMultiAppTestSuite{
			GenCaller: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgredis.NewBroker(dgredis.DefaultRedisConfig())
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
				brk, err := dgredis.NewBroker(dgredis.DefaultRedisConfig())
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
