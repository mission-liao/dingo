package dgredis

import (
	"errors"
	"fmt"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

var _redisTaskQueue = "dingo.tasks"

//
// core component
//

type broker struct {
	pool      *redis.Pool
	listeners *common.Routines
	cfg       RedisConfig
}

func NewBroker(cfg *RedisConfig) (v *broker, err error) {
	v = &broker{
		listeners: common.NewRoutines(),
		cfg:       *cfg,
	}

	v.pool, err = newRedisPool(&v.cfg)
	if err != nil {
		return
	}

	return
}

//
// common.Object interface
//

func (me *broker) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.listeners.Events(),
	}, nil
}

func (me *broker) Close() (err error) {
	me.listeners.Close()
	err = me.pool.Close()

	return
}

//
// Producer interface
//

func (me *broker) ProducerHook(eventID int, payload interface{}) (err error) {
	return
}

func (me *broker) Send(id transport.Meta, body []byte) (err error) {
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

func (me *broker) AddListener(name string, receipts <-chan *dingo.TaskReceipt) (tasks <-chan []byte, err error) {
	t := make(chan []byte, 10)
	go me._consumer_routine_(me.listeners.New(), me.listeners.Wait(), me.listeners.Events(), t, receipts, name)

	tasks = t
	return
}

func (me *broker) StopAllListeners() (err error) {
	me.listeners.Close()
	return
}

//
// routine definitions
//

func (me *broker) _consumer_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *common.Event,
	tasks chan<- []byte,
	receipts <-chan *dingo.TaskReceipt,
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
			reply, err := conn.Do("BRPOP", qn, me.cfg.GetListenTimeout())
			if err != nil {
				events <- common.NewEventFromError(dingo.InstT.CONSUMER, err)
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
					dingo.InstT.CONSUMER,
					errors.New(fmt.Sprintf("invalid reply: %v", reply)),
				)
				break
			}
			if len(v) != 2 {
				events <- common.NewEventFromError(
					dingo.InstT.CONSUMER,
					errors.New(fmt.Sprintf("invalid reply: %v", reply)),
				)
				break
			}

			b, ok := v[1].([]byte)
			if !ok {
				events <- common.NewEventFromError(
					dingo.InstT.CONSUMER,
					errors.New(fmt.Sprintf("invalid reply: %v", reply)),
				)
				break
			}

			h, err := transport.DecodeHeader(b)
			if err != nil {
				events <- common.NewEventFromError(dingo.InstT.CONSUMER, err)
				break
			}

			tasks <- b
			rcpt, ok := <-receipts
			if !ok {
				goto clean
			}

			if rcpt.ID != h.ID() {
				events <- common.NewEventFromError(
					dingo.InstT.CONSUMER,
					errors.New(fmt.Sprintf("expected: %v, received: %v", h, rcpt)),
				)
				break
			}
		}
	}
clean:
	return
}
