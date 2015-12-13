package dgredis

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type RedisBackendTestSuite struct {
	dingo.BackendTestSuite
}

func (me *RedisBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	me.Bkd, err = NewBackend(DefaultRedisConfig())
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func (me *RedisBackendTestSuite) TearDownSuite() {
	me.BackendTestSuite.TearDownSuite()
}

func TestRedisBackendSuite(t *testing.T) {
	suite.Run(t, &RedisBackendTestSuite{})
}
