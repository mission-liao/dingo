package broker

import (
	"errors"
	"fmt"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
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
// common.Object interface
//

func (me *_redis) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.listeners.Events(),
	}, nil
}

func (me *_redis) Close() (err error) {
	me.listeners.Close()
	err = me.pool.Close()

	return
}

//
// Producer interface
//

func (me *_redis) Send(id transport.Meta, body []byte) (err error) {
	conn := me.pool.Get()
	defer conn.Close()

	_, err = conn.Do("LPUSH", fmt.Sprintf("%v.%v", _redisTaskQueue, id.Name()), body)
	if err != nil {
		return
	}

	return
}

//
// Consumer interface
//

func (me *_redis) AddListener(name string, receipts <-chan *Receipt) (tasks <-chan []byte, err error) {
	t := make(chan []byte, 10)
	go me._consumer_routine_(me.listeners.New(), me.listeners.Wait(), me.listeners.Events(), t, receipts, name)

	tasks = t
	return
}

func (me *_redis) StopAllListeners() (err error) {
	me.listeners.Close()
	return
}

//
// routine definitions
//

func (me *_redis) _consumer_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *common.Event,
	tasks chan<- []byte,
	receipts <-chan *Receipt,
	name string,
) {
	defer wait.Done()

	conn := me.pool.Get()
	defer conn.Close()

	qn := fmt.Sprintf("%v.%v", _redisTaskQueue, name)

	for {
		select {
		case _, _ = <-quit:
			goto clean
		default:
			// blocking call on redis server
			reply, err := conn.Do("BRPOP", qn, 1) // TODO: configuration, in seconds
			if err != nil {
				events <- common.NewEventFromError(common.InstT.CONSUMER, err)
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
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("invalid reply: %v", reply)),
				)
				break
			}
			if len(v) != 2 {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("invalid reply: %v", reply)),
				)
				break
			}

			b, ok := v[1].([]byte)
			if !ok {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("invalid reply: %v", reply)),
				)
				break
			}

			h, err := transport.DecodeHeader(b)
			if err != nil {
				events <- common.NewEventFromError(common.InstT.CONSUMER, err)
				break
			}

			tasks <- b
			rcpt, ok := <-receipts
			if !ok {
				goto clean
			}

			if rcpt.ID != h.ID() {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("expected: %v, received: %v", h, rcpt)),
				)
				break
			}
		}
	}
clean:
	return
}
