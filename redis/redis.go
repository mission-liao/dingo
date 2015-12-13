package dgredis

import (
	"github.com/garyburd/redigo/redis"
	"time"
)

//
// A redis pool
//

func NewRedisPool(server, password string) (pool *redis.Pool, err error) {
	return &redis.Pool{
		MaxIdle:     3,                 // TODO: configuration
		IdleTimeout: 240 * time.Second, // TODO: configuration
		Dial: func() (conn redis.Conn, err error) {
			conn, err = redis.Dial("tcp", server)
			if err != nil {
				return
			}

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
