package broker

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

var _redisTaskQueue = "dingo.tasks"

type _redisConfig struct {
	common.RedisConfig
}

func defaultRedisConfig() *_redisConfig {
	return &_redisConfig{
		RedisConfig: *common.DefaultRedisConfig(),
	}
}

//
// core component
//

type _redis struct {
	pool      *redis.Pool
	listeners *common.Routines
}

func newRedis(cfg *Config) (v *_redis, err error) {
	v = &_redis{
		listeners: common.NewRoutines(),
	}

	v.pool, err = common.NewRedisPool(cfg.Redis.Connection(), cfg.Redis.Password_)
	if err != nil {
		return
	}

	return
}

//
// common.Server interface
//

func (me *_redis) Close() (err error) {
	me.listeners.Close()
	err = me.pool.Close()

	return
}

//
// Producer interface
//

func (me *_redis) Send(t meta.Task) (err error) {
	body, err := json.Marshal(t)
	if err != nil {
		return
	}

	conn := me.pool.Get()
	_, err = conn.Do("LPUSH", _redisTaskQueue, body)
	if err != nil {
		return
	}

	return
}

//
// Consumer interface
//

func (me *_redis) AddListener(receipts <-chan Receipt) (<-chan meta.Task, <-chan error, error) {
	tasks := make(chan meta.Task, 10)
	errs := make(chan error, 10)
	quit, wait := me.listeners.New()
	go me._consumer_routine_(quit, wait, tasks, errs, receipts)

	return tasks, errs, nil
}

func (me *_redis) Stop() (err error) {
	me.listeners.Close()
	return
}

//
// routine definitions
//

func (me *_redis) _consumer_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	tasks chan<- meta.Task,
	errs chan<- error,
	receipts <-chan Receipt,
) {
	defer wait.Done()

	conn := me.pool.Get()
	defer conn.Close()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		default:
			// blocking call on redis server
			reply, err := conn.Do("BRPOP", _redisTaskQueue, 1) // TODO: configuration, in seconds
			if err != nil {
				errs <- err
				break
			}

			if reply == nil {
				// timeout, go for next round
				break
			}

			// the return value from BRPOP should be
			// an slice with length 2
			v, ok := reply.([]interface{})
			if !ok {
				errs <- errors.New(fmt.Sprintf("invalid reply: %v", reply))
				break
			}
			if len(v) != 2 {
				errs <- errors.New(fmt.Sprintf("invalid reply: %v", reply))
				break
			}

			b, ok := v[1].([]byte)
			if !ok {
				errs <- errors.New(fmt.Sprintf("invalid reply: %v", reply))
				break
			}

			t, err := meta.UnmarshalTask(b)
			if err != nil {
				errs <- err
				break
			}

			tasks <- t
		}
	}
cleanup:
	return
}
