package dgredis

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type RedisBackendTestSuite struct {
	dingo.BackendTestSuite
}

func TestRedisBackendSuite(t *testing.T) {
	suite.Run(t, &RedisBackendTestSuite{
		dingo.BackendTestSuite{
			Gen: func() (b dingo.Backend, err error) {
				b, err = NewBackend(DefaultRedisConfig())
				return
			},
		},
	})
}
