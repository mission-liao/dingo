package backend

import (
	"errors"
	"fmt"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

var _redisResultQueue = "dingo.result"

type _redisConfig struct {
	common.RedisConfig
}

func defaultRedisConfig() *_redisConfig {
	return &_redisConfig{
		RedisConfig: *common.DefaultRedisConfig(),
	}
}

type _redis struct {
	pool *redis.Pool
	cfg  *Config

	// reporter
	reporters *common.HetroRoutines

	// store
	stores   *common.HetroRoutines
	rids     map[string]int
	ridsLock sync.Mutex
}

func newRedis(cfg *Config) (v *_redis, err error) {
	v = &_redis{
		reporters: common.NewHetroRoutines(),
		rids:      make(map[string]int),
		stores:    common.NewHetroRoutines(),
		cfg:       cfg,
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
		me.reporters.Events(),
		me.stores.Events(),
	}, nil
}

func (me *_redis) Close() (err error) {
	me.reporters.Close()
	me.stores.Close()
	err = me.pool.Close()
	return
}

//
// Reporter interface
//

func (me *_redis) Report(reports <-chan *Envelope) (id int, err error) {
	quit, done, id := me.reporters.New(0)
	go me._reporter_routine_(quit, done, me.reporters.Events(), reports)

	return
}

//
// Store interface
//

func (me *_redis) Poll(id transport.Meta) (reports <-chan []byte, err error) {
	quit, done, idx := me.stores.New(0)

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	me.rids[id.ID()] = idx

	r := make(chan []byte, 10)
	reports = r
	go me._store_routine_(quit, done, me.stores.Events(), r, id)

	return
}

func (me *_redis) Done(id transport.Meta) (err error) {
	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()

	v, ok := me.rids[id.ID()]
	if !ok {
		err = errors.New("store id not found")
		return
	}

	delete(me.rids, id.ID())
	return me.stores.Stop(v)
}

//
// routine definition
//

func (me *_redis) _reporter_routine_(quit <-chan int, done chan<- int, events chan<- *common.Event, reports <-chan *Envelope) {
	var (
		err error
	)
	defer func() {
		done <- 1
	}()

	conn := me.pool.Get()
	defer conn.Close()
	for {
		select {
		case _, _ = <-quit:
			goto clean
		case e, ok := <-reports:
			if !ok {
				goto clean
			}

			_, err = conn.Do("LPUSH", getKey(e.ID), e.Body)
			if err != nil {
				events <- common.NewEventFromError(common.InstT.REPORTER, err)
				break
			}
		}
	}
clean:
}

func (me *_redis) _store_routine_(quit <-chan int, done chan<- int, events chan<- *common.Event, reports chan<- []byte, id transport.Meta) {
	conn := me.pool.Get()
	defer func() {
		done <- 1

		// delete key in redis
		_, err := conn.Do("DEL", getKey(id))
		if err != nil {
			events <- common.NewEventFromError(common.InstT.STORE, err)
		}

		err = conn.Close()
		if err != nil {
			events <- common.NewEventFromError(common.InstT.STORE, err)
		}
	}()

	for {
		select {
		case _, _ = <-quit:
			goto clean
		default:
			// blocking call to redis
			reply, err := conn.Do("BRPOP", getKey(id), 1) // TODO: configuration, in seconds
			if err != nil {
				events <- common.NewEventFromError(common.InstT.STORE, err)
				break
			}
			if reply == nil {
				// timeout
				break
			}

			v, ok := reply.([]interface{})
			if !ok {
				events <- common.NewEventFromError(
					common.InstT.STORE,
					errors.New(fmt.Sprintf("Unable to get array of interface{} from %v", reply)),
				)
				break
			}
			if len(v) != 2 {
				events <- common.NewEventFromError(
					common.InstT.STORE,
					errors.New(fmt.Sprintf("length of reply is not 2, but %v", v)),
				)
				break
			}

			b, ok := v[1].([]byte)
			if !ok {
				events <- common.NewEventFromError(
					common.InstT.STORE,
					errors.New(fmt.Sprintf("the first object of reply is not byte-array, but %v", v)),
				)
				break
			}

			reports <- b
		}
	}
clean:
}

//
// private function
//

func getKey(id transport.Meta) string {
	return fmt.Sprintf("%v.%d", _redisResultQueue, id.ID())
}
