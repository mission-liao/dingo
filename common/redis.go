package common

import (
	"github.com/garyburd/redigo/redis"

	"fmt"
	"time"
)

type RedisConfig struct {
	Host_     string `json:"Host"`
	Port_     int    `json:"Port"`
	Password_ string `json:"Password"`
}

func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host_:     "localhost",
		Port_:     6379,
		Password_: "",
	}
}

//
// setter
//

func (me *RedisConfig) Host(host string) *RedisConfig {
	me.Host_ = host
	return me
}

func (me *RedisConfig) Port(port int) *RedisConfig {
	me.Port_ = port
	return me
}

func (me *RedisConfig) Password(password string) *RedisConfig {
	me.Password_ = password
	return me
}

//
// getter
//

func (me *RedisConfig) Connection() string {
	return fmt.Sprintf("%v:%v", me.Host_, me.Port_)
}

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
