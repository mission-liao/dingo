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
	DingoSingleAppTestSuite
}

func TestDingoRedisSingleAppSuite(t *testing.T) {
	suite.Run(t, &redisSingleAppTestSuite{
		DingoSingleAppTestSuite{
			GenApp: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgredis.NewBroker(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(brk, dingo.ObjT.Default)
				if err != nil {
					return
				}

				bkd, err := dgredis.NewBackend(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(bkd, dingo.ObjT.Default)
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
	DingoMultiAppTestSuite
}

func TestDingoRedisMultiAppSuite(t *testing.T) {
	suite.Run(t, &redisMultiAppTestSuite{
		DingoMultiAppTestSuite{
			CountOfCallers: 3,
			CountOfWorkers: 3,
			GenCaller: func() (app *dingo.App, err error) {
				app, err = dingo.NewApp("remote", nil)
				if err != nil {
					return
				}
				brk, err := dgredis.NewBroker(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(brk, dingo.ObjT.Producer)
				if err != nil {
					return
				}
				bkd, err := dgredis.NewBackend(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(bkd, dingo.ObjT.Store)
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
				_, _, err = app.Use(brk, dingo.ObjT.NamedConsumer)
				if err != nil {
					return
				}
				bkd, err := dgredis.NewBackend(dgredis.DefaultRedisConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(bkd, dingo.ObjT.Reporter)
				if err != nil {
					return
				}

				return
			},
		},
	})
}
