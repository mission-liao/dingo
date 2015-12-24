package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/amqp"
	"github.com/stretchr/testify/suite"
)

//
// Amqp(Broker) + Amqp(Backend), Single App
//

type amqpSingleAppTestSuite struct {
	dingo.DingoSingleAppTestSuite
}

func TestDingoAmqpSingleAppSuite(t *testing.T) {
	suite.Run(t, &amqpSingleAppTestSuite{
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
				_, _, err = app.Use(brk, dingo.ObjT.Default)
				if err != nil {
					return
				}

				bkd, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
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
// Amqp(Broker) + Amqp(Backend), Multi App
//

type amqpMultiAppTestSuite struct {
	dingo.DingoMultiAppTestSuite
}

func TestDingoAmqpMultiAppSuite(t *testing.T) {
	suite.Run(t, &amqpMultiAppTestSuite{
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
				_, _, err = app.Use(brk, dingo.ObjT.Producer)
				if err != nil {
					return
				}
				bkd, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
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
				brk, err := dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
				if err != nil {
					return
				}
				_, _, err = app.Use(brk, dingo.ObjT.NamedConsumer)
				if err != nil {
					return
				}
				bkd, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
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
