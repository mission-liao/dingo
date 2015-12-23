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
				_, _, err = app.Use(brk, dingo.ObjT.DEFAULT)
				if err != nil {
					return
				}

				bkd, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
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
// Amqp(Broker) + Amqp(Backend), Multi App
//

type amqpMultiAppTestSuite struct {
	dingo.DingoMultiAppTestSuite
}

func TestDingoAmqpMultiAppSuite(t *testing.T) {
	suite.Run(t, &amqpMultiAppTestSuite{
		dingo.DingoMultiAppTestSuite{
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
				bkd, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
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
				bkd, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
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
