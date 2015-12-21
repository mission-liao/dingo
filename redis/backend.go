package dgredis

import (
	"errors"
	"fmt"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/mission-liao/dingo"
)

var _redisResultQueue = "dingo.result"

type backend struct {
	pool *redis.Pool
	cfg  RedisConfig

	// reporter
	reporters *dingo.HetroRoutines

	// store
	stores   *dingo.HetroRoutines
	rids     map[string]map[string]int
	ridsLock sync.Mutex
}

func NewBackend(cfg *RedisConfig) (v *backend, err error) {
	v = &backend{
		reporters: dingo.NewHetroRoutines(),
		rids:      make(map[string]map[string]int),
		stores:    dingo.NewHetroRoutines(),
		cfg:       *cfg,
	}
	v.pool, err = newRedisPool(&v.cfg)
	if err != nil {
		return
	}

	return
}

//
// dingo.Object interface
//

func (me *backend) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		me.reporters.Events(),
		me.stores.Events(),
	}, nil
}

func (me *backend) Close() (err error) {
	me.reporters.Close()
	me.stores.Close()
	err = me.pool.Close()
	return
}

//
// Reporter interface
//

func (me *backend) ReporterHook(eventID int, payload interface{}) (err error) {
	return
}

func (me *backend) Report(reports <-chan *dingo.ReportEnvelope) (id int, err error) {
	quit, done, id := me.reporters.New(0)
	go me._reporter_routine_(quit, done, me.reporters.Events(), reports)

	return
}

//
// Store interface
//

func (me *backend) Poll(meta dingo.Meta) (reports <-chan []byte, err error) {
	quit, done, idx := me.stores.New(0)

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	if v, ok := me.rids[meta.Name()]; ok {
		v[meta.ID()] = idx
	} else {
		me.rids[meta.Name()] = map[string]int{meta.ID(): idx}
	}

	r := make(chan []byte, 10)
	reports = r
	go me._store_routine_(quit, done, me.stores.Events(), r, meta)

	return
}

func (me *backend) Done(meta dingo.Meta) (err error) {
	var v int
	err = func() (err error) {
		var (
			ok  bool
			ids map[string]int
		)

		me.ridsLock.Lock()
		defer me.ridsLock.Unlock()

		if ids, ok = me.rids[meta.Name()]; ok {
			v, ok = ids[meta.ID()]
			delete(ids, meta.ID())
		}
		if !ok {
			err = errors.New("store id not found")
			return
		}

		return
	}()
	if err == nil {
		err = me.stores.Stop(v)
	}
	return
}

//
// routine definition
//

func (me *backend) _reporter_routine_(quit <-chan int, done chan<- int, events chan<- *dingo.Event, reports <-chan *dingo.ReportEnvelope) {
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
				events <- dingo.NewEventFromError(dingo.InstT.REPORTER, err)
				break
			}
		}
	}
clean:
}

func (me *backend) _store_routine_(quit <-chan int, done chan<- int, events chan<- *dingo.Event, reports chan<- []byte, id dingo.Meta) {
	conn := me.pool.Get()
	defer func() {
		done <- 1

		// delete key in redis
		_, err := conn.Do("DEL", getKey(id))
		if err != nil {
			events <- dingo.NewEventFromError(dingo.InstT.STORE, err)
		}

		err = conn.Close()
		if err != nil {
			events <- dingo.NewEventFromError(dingo.InstT.STORE, err)
		}
	}()

finished:
	for {
		select {
		case _, _ = <-quit:
			break finished
		default:
			// blocking call to redis
			reply, err := conn.Do("BRPOP", getKey(id), me.cfg.GetPollTimeout())
			if err != nil {
				events <- dingo.NewEventFromError(dingo.InstT.STORE, err)
				break
			}
			if reply == nil {
				// timeout
				break
			}

			v, ok := reply.([]interface{})
			if !ok {
				events <- dingo.NewEventFromError(
					dingo.InstT.STORE,
					errors.New(fmt.Sprintf("Unable to get array of interface{} from %v", reply)),
				)
				break
			}
			if len(v) != 2 {
				events <- dingo.NewEventFromError(
					dingo.InstT.STORE,
					errors.New(fmt.Sprintf("length of reply is not 2, but %v", v)),
				)
				break
			}

			b, ok := v[1].([]byte)
			if !ok {
				events <- dingo.NewEventFromError(
					dingo.InstT.STORE,
					errors.New(fmt.Sprintf("the first object of reply is not byte-array, but %v", v)),
				)
				break
			}

			reports <- b
		}
	}
}

//
// private function
//

func getKey(meta dingo.Meta) string {
	return fmt.Sprintf("%v.%s.%s", _redisResultQueue, meta.Name(), meta.ID())
}
