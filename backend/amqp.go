package backend

import (
	// standard
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	// open source
	"github.com/streadway/amqp"

	// internal
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

type _amqpConfig struct {
	common.AmqpConfig

	Reporters_ int `json:"Reporters"`
}

func defaultAmqpConfig() *_amqpConfig {
	return &_amqpConfig{
		AmqpConfig: *common.DefaultAmqpConfig(),
		Reporters_: 3,
	}
}

func (me *_amqpConfig) Reporters(count int) *_amqpConfig {
	me.Reporters_ = count
	return me
}

type _amqp struct {
	common.AmqpConnection

	// store
	reports  chan meta.Report
	monitors *common.HetroRoutines
	rids     map[string]int
	ridsLock sync.Mutex

	// reporter
	reporters     *common.Routines
	reportersLock sync.Mutex
	cfg           *Config
}

func newAmqp(cfg *Config) (v *_amqp, err error) {
	v = &_amqp{
		reporters: common.NewRoutines(),
		rids:      make(map[string]int),
		cfg:       cfg,
		reports:   make(chan meta.Report, 10), // TODO: config
		monitors:  common.NewHetroRoutines(),
	}
	err = v.init()
	return
}

func (me *_amqp) init() (err error) {
	// call parent's Init
	err = me.AmqpConnection.Init(me.cfg.AMQP_().Connection())
	if err != nil {
		return
	}

	// define exchange
	ci, err := me.AmqpConnection.Channel()
	if err != nil {
		return
	}
	defer me.AmqpConnection.ReleaseChannel(ci)

	// init exchange
	err = ci.Channel.ExchangeDeclare(
		"dingo.x.result", // name of exchange
		"direct",         // kind
		true,             // durable
		false,            // auto-delete
		false,            // internal
		false,            // noWait
		nil,              // args
	)
	if err != nil {
		return
	}

	return
}

//
// common.Server interface
//

func (me *_amqp) Close() (err error) {
	err = me.AmqpConnection.Close()
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

func (me *_amqp) Report(reports <-chan meta.Report) (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	remain := me.cfg._amqp.Reporters_
	for ; remain > 0; remain-- {
		quit, wait := me.reporters.New()
		go me._reporter_routine_(quit, wait, reports)
	}

	if remain > 0 {
		err = errors.New(fmt.Sprintf("Still %v reporters uninitiated", remain))
	}
	return
}

func (me *_amqp) Unbind() (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	me.reporters.Close()
	return
}

//
// Store interface
//

func (me *_amqp) Subscribe() (reports <-chan meta.Report, err error) {
	return me.reports, nil
}

func (me *_amqp) Poll(id meta.ID) (err error) {
	// bind to the queue for this task
	tag, qName, rKey := getConsumerTag(id), getQueueName(id), getRoutingKey(id)
	quit, done, idx := me.monitors.New()

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	me.rids[id.GetId()] = idx

	// acquire a free channel
	ci, err := me.AmqpConnection.Channel()
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			me.AmqpConnection.ReleaseChannel(ci)
		}
	}()
	// declare a queue for this task
	_, err = ci.Channel.QueueDeclare(
		qName, // name of queue
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // nowait
		nil,   // args
	)
	if err != nil {
		return
	}

	// bind queue to result-exchange
	err = ci.Channel.QueueBind(
		qName,            // name of queue
		rKey,             // routing key
		"dingo.x.result", // name of exchange
		false,            // nowait
		nil,              // args
	)
	if err != nil {
		return
	}

	dv, err := ci.Channel.Consume(
		qName, // name of queue
		tag,   // consumer Tag
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return
	}

	go me._store_routine_(quit, done, me.reports, ci, dv, id)
	return
}

func (me *_amqp) Done(id meta.ID) (err error) {
	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()

	v, ok := me.rids[id.GetId()]
	if !ok {
		err = errors.New("monitor id not found")
		return
	}

	return me.monitors.Stop(v)
}

//
// routine definition
//

func (me *_amqp) _reporter_routine_(quit <-chan int, wait *sync.WaitGroup, reports <-chan meta.Report) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case r, ok := <-reports:
			if !ok {
				// reports channel is closed
				goto cleanup
			}

			// TODO: errs channel
			body, _ := json.Marshal(r)

			// TODO: errs channel
			// acquire a channel
			_ = func() (err error) {
				ci, err := me.AmqpConnection.Channel()
				if err != nil {
					return
				}
				defer me.AmqpConnection.ReleaseChannel(ci)

				qName, rKey := getQueueName(r), getRoutingKey(r)

				// declare a queue for this task
				_, err = ci.Channel.QueueDeclare(
					qName, // name of queue
					true,  // durable
					false, // auto-delete
					false, // exclusive
					false, // nowait
					nil,   // args
				)
				if err != nil {
					return
				}

				// bind queue to result-exchange
				err = ci.Channel.QueueBind(
					qName,            // name of queue
					rKey,             // routing key
					"dingo.x.result", // name of exchange
					false,            // nowait
					nil,              // args
				)
				if err != nil {
					return
				}

				err = ci.Channel.Publish(
					"dingo.x.result", // name of exchange
					rKey,             // routing key
					false,            // madatory
					false,            // immediate
					amqp.Publishing{
						DeliveryMode: amqp.Persistent,
						ContentType:  "text/json",
						Body:         body,
					},
				)
				if err != nil {
					return
				}

				// block until amqp.Channel.NotifyPublish
				// TODO: time out, retry
				cf := <-ci.Confirm
				if !cf.Ack {
					err = errors.New("Unable to publish to server")
				}

				return
			}()
		}
	}

cleanup:
	// TODO: cosume all remaining reports?
}

func (me *_amqp) _store_routine_(
	quit <-chan int,
	done chan<- int,
	reports chan<- meta.Report,
	ci *common.AmqpChannel,
	dv <-chan amqp.Delivery,
	id meta.ID) {

	var err error

	defer func() {
		me.AmqpConnection.ReleaseChannel(ci)
		done <- 1
		close(done)
	}()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case d, ok := <-dv:
			if !ok {
				goto cleanup
			}

			r, err := meta.UnmarshalReport(d.Body)
			if err != nil {
				d.Nack(false, false)
				// TODO: errs channel
				break
			}

			reports <- r
		}
	}

cleanup:
	// cancel consuming
	err = ci.Channel.Cancel(getConsumerTag(id), false)
	if err != nil {
		// TODO: errs channel
		return
	}

	// unbind queue from exchange
	qName, rKey := getQueueName(id), getRoutingKey(id)
	err = ci.Channel.QueueUnbind(
		qName,            // name of queue
		rKey,             // routing key
		"dingo.x.result", // name of exchange
		nil,              // args
	)
	if err != nil {
		// TODO: errs channel
		return
	}

	// delete queue
	_, err = ci.Channel.QueueDelete(
		qName, // name of queue
		true,  // ifUnused
		true,  // ifEmpty
		false, // noWait
	)
	if err != nil {
		// TODO: errs channel
		return
	}
}

//
// private function
//

//
func getQueueName(id meta.ID) string {
	return fmt.Sprintf("dingo.q.%q", id.GetId())
}

//
func getRoutingKey(id meta.ID) string {
	return fmt.Sprintf("dingo.rkey.%q", id.GetId())
}

//
func getConsumerTag(id meta.ID) string {
	return fmt.Sprintf("dingo.consumer.%q", id.GetId())
}
