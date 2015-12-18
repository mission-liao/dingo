package dgredis

import (
	"fmt"
	"time"
)

type RedisConfig struct {
	Host_          string        `json:"Host"`
	Port_          int           `json:"Port"`
	Password_      string        `json:"Password"`
	PollTimeout_   int           `json:"PollTimeout"`
	ListenTimeout_ int           `json:"ListenTimeout"`
	MaxIdle_       int           `json:"MaxIdle"`
	IdleTimeout_   time.Duration `json:"IdleTimeout"`
}

func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host_:          "localhost",
		Port_:          6379,
		Password_:      "",
		PollTimeout_:   1,
		ListenTimeout_: 1,
		MaxIdle_:       3,
		IdleTimeout_:   240 * time.Second,
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

func (me *RedisConfig) PollTimeout(timeout int) *RedisConfig {
	me.PollTimeout_ = timeout
	return me
}

func (me *RedisConfig) ListenTimeout(timeout int) *RedisConfig {
	me.ListenTimeout_ = timeout
	return me
}

func (me *RedisConfig) MaxIdle(count int) *RedisConfig {
	me.MaxIdle_ = count
	return me
}

func (me *RedisConfig) IdleTimeout(timeout time.Duration) *RedisConfig {
	me.IdleTimeout_ = timeout
	return me
}

//
// getter
//

func (me *RedisConfig) Connection() string {
	return fmt.Sprintf("%v:%v", me.Host_, me.Port_)
}

func (me *RedisConfig) GetPassword() string {
	return me.Password_
}

func (me *RedisConfig) GetListenTimeout() int {
	return me.ListenTimeout_
}

func (me *RedisConfig) GetPollTimeout() int {
	return me.PollTimeout_
}

func (me *RedisConfig) GetMaxIdle() int {
	return me.MaxIdle_
}

func (me *RedisConfig) GetIdleTimeout() time.Duration {
	return me.IdleTimeout_
}
