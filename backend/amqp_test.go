package backend

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AmqpBackendTestSuite struct {
	BackendTestSuite
}

func (me *AmqpBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	me._backend, err = New("amqp", cfg)
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func (me *AmqpBackendTestSuite) TearDownSuite() {
	me.BackendTestSuite.TearDownSuite()
}

func TestAmqpBackendSuite(t *testing.T) {
	suite.Run(t, &AmqpBackendTestSuite{})
}
