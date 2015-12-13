package tests

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/amqp"
	"github.com/stretchr/testify/suite"
)

//
// Amqp(Broker) + Amqp(Backend)
//

type amqpTestSuite struct {
	dingo.DingoTestSuite
}

func (me *amqpTestSuite) SetupSuite() {
	var err error

	// broker
	me.Nbrk, err = dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
	me.Nil(err)

	// backend
	me.Bkd, err = dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
	me.Nil(err)

	me.DingoTestSuite.SetupSuite()
}

func TestDingoAmqpSuite(t *testing.T) {
	suite.Run(t, &amqpTestSuite{})
}
