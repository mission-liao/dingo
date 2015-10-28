package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
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
	reporters     *common.Routines
	reportersLock sync.Mutex

	// store
	reports  chan meta.Report
	monitors *common.HetroRoutines
	rids     map[string]int
	ridsLock sync.Mutex
}

func newRedis(cfg *Config) (v *_redis, err error) {
	v = &_redis{
		reporters: common.NewRoutines(),
		reports:   make(chan meta.Report, 10),
		rids:      make(map[string]int),
		monitors:  common.NewHetroRoutines(),
		cfg:       cfg,
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
	me.reporters.Close()
	err = me.pool.Close()
	err_ := me.Unbind()
	if err == nil {
		err = err_
	}
	me.monitors.Close()

	return
}

//
// Reporter interface
//

func (me *_redis) Report(reports <-chan meta.Report) (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	remain := me.cfg.Reporters_
	for ; remain > 0; remain-- {
		quit, wait := me.reporters.New()
		go me._reporter_routine_(quit, wait, reports)
	}

	if remain > 0 {
		err = errors.New(fmt.Sprintf("Still %v reporters uninitiated", remain))
	}
	return
}

func (me *_redis) Unbind() (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	me.reporters.Close()
	return
}

//
// Store interface
//

func (me *_redis) Subscribe() (reports <-chan meta.Report, err error) {
	return me.reports, nil
}

func (me *_redis) Poll(id meta.ID) (err error) {
	quit, done, idx := me.monitors.New()

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	me.rids[id.GetId()] = idx

	go me._monitor_routine_(quit, done, me.reports, id)

	return
}

func (me *_redis) Done(id meta.ID) (err error) {
	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()

	v, ok := me.rids[id.GetId()]
	if !ok {
		err = errors.New("monitor id not found")
		return
	}

	// TODO: delete key

	return me.monitors.Stop(v)
}

//
// routine definition
//

func (me *_redis) _reporter_routine_(quit <-chan int, wait *sync.WaitGroup, reports <-chan meta.Report) {
	defer wait.Done()

	conn := me.pool.Get()
	defer conn.Close()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case r, ok := <-reports:
			if !ok {
				goto cleanup
			}

			// TODO: errs channel
			body, _ := json.Marshal(r)

			// TODO: errs channel
			conn.Do("LPUSH", getKey(r), body)
		}
	}
cleanup:
	return
}

func (me *_redis) _monitor_routine_(
	quit <-chan int,
	done chan<- int,
	reports chan<- meta.Report,
	id meta.ID) {

	defer func() {
		done <- 1
	}()

	conn := me.pool.Get()
	defer conn.Close()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		default:
			// TODO: errs channel
			// blocking call to redis
			reply, _ := conn.Do("BRPOP", getKey(id), 1) // TODO: configuration, in seconds
			if reply == nil {
				// timeout
				break
			}

			v, ok := reply.([]interface{})
			if !ok {
				break
			}
			if len(v) != 2 {
				break
			}

			b, ok := v[1].([]byte)
			if !ok {
				break
			}

			r, _ := meta.UnmarshalReport(b)
			reports <- r
		}
	}
cleanup:
}

//
// private function
//

func getKey(id meta.ID) string {
	return fmt.Sprintf("%v.%d", _redisResultQueue, id.GetId())
}
