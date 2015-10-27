package backend

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RedisBackendTestSuite struct {
	BackendTestSuite
}

func (me *RedisBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	me._backend, err = New("redis", cfg)
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func (me *RedisBackendTestSuite) TearDownSuite() {
	me.BackendTestSuite.TearDownSuite()
}

func TestRedisBackendSuite(t *testing.T) {
	suite.Run(t, &RedisBackendTestSuite{})
}
