package dgredis

import (
	"fmt"
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
