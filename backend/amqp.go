package backend

import (
	// standard
	"errors"
	"fmt"
	"sync"

	// open source
	"github.com/streadway/amqp"

	// internal
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

type _amqpConfig struct {
	common.AmqpConfig
}

func defaultAmqpConfig() *_amqpConfig {
	return &_amqpConfig{
		AmqpConfig: *common.DefaultAmqpConfig(),
	}
}

type _amqp struct {
	common.AmqpConnection

	// store
	stores   *common.HetroRoutines
	rids     map[string]int
	ridsLock sync.Mutex

	// reporter
	reporters *common.HetroRoutines
	cfg       *Config
}

func newAmqp(cfg *Config) (v *_amqp, err error) {
	v = &_amqp{
		reporters: common.NewHetroRoutines(),
		rids:      make(map[string]int),
		cfg:       cfg,
		stores:    common.NewHetroRoutines(),
	}
	err = v.init()
	return
}

func (me *_amqp) init() (err error) {
	// call parent's Init
	err = me.AmqpConnection.Init(me.cfg.Amqp.Connection())
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
// common.Object interface
//

func (me *_amqp) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.reporters.Events(),
		me.stores.Events(),
	}, nil
}

func (me *_amqp) Close() (err error) {
	err = me.reporters.Close()
	err2 := me.stores.Close()
	if err == nil {
		err = err2
	}
	err2 = me.AmqpConnection.Close()
	if err == nil {
		err = err2
	}
	return
}

//
// Reporter interface
//

func (me *_amqp) Report(reports <-chan *Envelope) (id int, err error) {
	quit, done, id := me.reporters.New(0)
	go me._reporter_routine_(quit, done, me.reporters.Events(), reports)
	return
}

//
// Store interface
//

func (me *_amqp) Poll(id transport.Meta) (reports <-chan []byte, err error) {
	// bind to the queue for this task
	tag, qName, rKey := getConsumerTag(id), getQueueName(id), getRoutingKey(id)
	quit, done, idx := me.stores.New(0)

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	me.rids[id.ID()] = idx

	// acquire a free channel
	ci, err := me.AmqpConnection.Channel()
	if err != nil {
		return
	}
	var (
		dv <-chan amqp.Delivery
		r  chan []byte
	)
	defer func() {
		if err != nil {
			me.AmqpConnection.ReleaseChannel(ci)
		} else {
			go me._store_routine_(quit, done, me.stores.Events(), r, ci, dv, id)
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

	dv, err = ci.Channel.Consume(
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

	r = make(chan []byte, 10)
	reports = r
	return
}

func (me *_amqp) Done(id transport.Meta) (err error) {
	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()

	v, ok := me.rids[id.ID()]
	if !ok {
		err = errors.New("store id not found")
		return
	}

	return me.stores.Stop(v)
}

//
// routine definition
//

func (me *_amqp) _reporter_routine_(quit <-chan int, done chan<- int, events chan<- *common.Event, reports <-chan *Envelope) {
	// TODO: keep one AmqpConnection, instead of get/realease for each report.
	defer func() {
		done <- 1
	}()

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case e, ok := <-reports:
			if !ok {
				// reports channel is closed
				goto clean
			}

			// acquire a channel
			err := func() (err error) {
				ci, err := me.AmqpConnection.Channel()
				if err != nil {
					return
				}
				defer me.AmqpConnection.ReleaseChannel(ci)

				qName, rKey := getQueueName(e.ID), getRoutingKey(e.ID)

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
						Body:         e.Body,
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
					return
				}

				return
			}()
			if err != nil {
				events <- common.NewEventFromError(common.InstT.REPORTER, err)
				break
			}
		}
	}

clean:
	// TODO: cosume all remaining reports?
}

func (me *_amqp) _store_routine_(
	quit <-chan int,
	done chan<- int,
	events chan<- *common.Event,
	reports chan<- []byte,
	ci *common.AmqpChannel,
	dv <-chan amqp.Delivery,
	id transport.Meta) {

	var (
		err            error
		isChannelError bool = false
	)

	defer func() {
		if isChannelError {
			// when error occurs, this channel is
			// automatically closed.
			me.AmqpConnection.ReleaseChannel(nil)
		} else {
			me.AmqpConnection.ReleaseChannel(ci)
		}
		done <- 1
		close(done)
		close(reports)
	}()

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case d, ok := <-dv:
			if !ok {
				goto clean
			}

			reports <- d.Body
		}
	}

clean:
	// TODO: consuming remaining stuffs in queue
	// cancel consuming
	err = ci.Channel.Cancel(getConsumerTag(id), false)
	if err != nil {
		events <- common.NewEventFromError(common.InstT.STORE, err)
		isChannelError = true
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
		events <- common.NewEventFromError(common.InstT.STORE, err)
		isChannelError = true
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
		events <- common.NewEventFromError(common.InstT.STORE, err)
		isChannelError = true
		return
	}
}

//
// private function
//

//
func getQueueName(id transport.Meta) string {
	return fmt.Sprintf("dingo.q.%q", id.ID())
}

//
func getRoutingKey(id transport.Meta) string {
	return fmt.Sprintf("dingo.rkey.%q", id.ID())
}

//
func getConsumerTag(id transport.Meta) string {
	return fmt.Sprintf("dingo.consumer.%q", id.ID())
}
