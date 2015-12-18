package dgredis

import (
	"github.com/garyburd/redigo/redis"
	"time"
)

//
// A redis pool
//

func newRedisPool(cfg *RedisConfig) (pool *redis.Pool, err error) {
	return &redis.Pool{
		MaxIdle:     cfg.GetMaxIdle(),
		IdleTimeout: cfg.GetIdleTimeout(),
		Dial: func() (conn redis.Conn, err error) {
			conn, err = redis.Dial("tcp", cfg.Connection())
			if err != nil {
				return
			}

			password := cfg.GetPassword()
			if len(password) > 0 {
				_, err = conn.Do("AUTH", password)
				if err != nil {
					conn.Close()
					conn = nil
					return
				}
			}

			return
		},
		TestOnBorrow: func(conn redis.Conn, t time.Time) (err error) {
			_, err = conn.Do("PING")
			return
		},
	}, nil
}
