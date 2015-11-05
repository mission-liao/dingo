package broker

import (
	// standard
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	// open source
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/streadway/amqp"
)

type _amqpConfig struct {
	common.AmqpConfig
}

func defaultAmqpConfig() *_amqpConfig {
	return &_amqpConfig{
		AmqpConfig: *common.DefaultAmqpConfig(),
	}
}

//
// major component
//

type _amqp struct {
	sender, receiver *common.AmqpConnection
	consumerTags     chan int
	consumers        *common.Routines
}

func newAmqp(cfg *Config) (v *_amqp, err error) {
	v = &_amqp{
		sender:    &common.AmqpConnection{},
		receiver:  &common.AmqpConnection{},
		consumers: common.NewRoutines(),
	}

	conn := cfg.Amqp.Connection()
	err = v.sender.Init(conn)
	if err != nil {
		return
	}

	err = v.receiver.Init(conn)
	if err != nil {
		return
	}

	err = v.init()
	if err != nil {
		return
	}

	return
}

func (me *_amqp) init() (err error) {
	// define relation between queue and exchange

	// get a free channel,
	// either sender/receiver's channel would works
	ci, err := me.sender.Channel()
	if err != nil {
		return
	}

	// remember to return channel to pool
	defer me.sender.ReleaseChannel(ci)

	// init exchange
	err = ci.Channel.ExchangeDeclare(
		"dingo.x.task", // name of exchange
		"direct",       // kind
		true,           // durable
		false,          // auto-delete
		false,          // internal
		false,          // noWait
		nil,            // args
	)
	if err != nil {
		return
	}

	// init queue
	_, err = ci.Channel.QueueDeclare(
		"dingo.q.task", // name of queue
		true,           // durable
		false,          // auto-delete
		false,          // exclusive
		false,          // noWait
		nil,            // args
	)
	if err != nil {
		return
	}

	// bind queue to exchange
	// TODO: configurable name?
	err = ci.Channel.QueueBind(
		"dingo.q.task",
		"",
		"dingo.x.task",
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return
	}

	// init qos
	err = ci.Channel.Qos(3, 0, true)
	if err != nil {
		return
	}

	// TODO: get number from MaxChannel
	me.consumerTags = make(chan int, 200)
	for i := 0; i < 200; i++ {
		me.consumerTags <- i
	}

	return
}

//
// common.Object interface
//

func (me *_amqp) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.consumers.Events(),
	}, nil
}

func (me *_amqp) Close() (err error) {
	me.consumers.Close()

	err = me.sender.Close()
	err_ := me.receiver.Close()
	if err == nil && err_ != nil {
		// error from sender is propagated first
		err = err_
	}

	return
}

//
// Producer interface
//

func (me *_amqp) Send(t meta.Task) (err error) {
	// marshaling to json
	body, err := json.Marshal(t)
	if err != nil {
		return
	}

	// acquire a channel
	ci, err := me.sender.Channel()
	if err != nil {
		return
	}

	defer func(ci *common.AmqpChannel) {
		// release it when leaving this function
		me.sender.ReleaseChannel(ci)
	}(ci)

	err = ci.Channel.Publish(
		"dingo.x.task", // name of exchange
		"",             // routing key
		false,          // mandatory
		false,          // immediate
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
	select {
	case cf := <-ci.Confirm:
		if !cf.Ack {
			err = errors.New("Unable to publish to server")
		}
	case <-time.After(10 * time.Second):
		// TODO: configuration
		// TODO: add retry
		// time out
		err = errors.New("Server didn't reply 'Cancel' event in 10 seconds")
	}

	return
}

//
// Consumer interface
//

func (me *_amqp) AddListener(receipts <-chan Receipt) (tasks <-chan meta.Task, err error) {
	t := make(chan meta.Task, 10)
	quit := me.consumers.New()
	go me._consumer_routine_(quit, me.consumers.Wait(), me.consumers.Events(), t, receipts)

	tasks = t
	return
}

func (me *_amqp) Stop() (err error) {
	me.consumers.Close()
	return
}

//
// routine definitions
//

func (me *_amqp) _consumer_routine_(
	quit <-chan int,
	wait *sync.WaitGroup,
	events chan<- *common.Event,
	tasks chan<- meta.Task,
	receipts <-chan Receipt,
) {
	defer wait.Done()

	// acquire an tag
	id := <-me.consumerTags
	tag := fmt.Sprintf("dingo.consumer.%d", id)

	// return id
	defer func(id int) {
		me.consumerTags <- id
	}(id)

	// acquire a channel
	ci, err := me.receiver.Channel()
	if err != nil {
		events <- common.NewEventFromError(common.InstT.CONSUMER, err)
		return
	}
	defer me.receiver.ReleaseChannel(ci)

	dv, err := ci.Channel.Consume(
		"dingo.q.task",
		tag,   // consumer Tag
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		events <- common.NewEventFromError(common.InstT.CONSUMER, err)
		return
	}

	for {
		select {
		case d, ok := <-dv:
			if !ok {
				goto cleanup
			}
			t, err_ := meta.UnmarshalTask(d.Body)
			if err_ != nil {
				// an invalid task
				d.Nack(false, false)
				events <- common.NewEventFromError(common.InstT.CONSUMER, err_)
				break
			}

			tasks <- t
			reply, ok := <-receipts
			if !ok {
				goto cleanup
			}

			if reply.Status == Status.WORKER_NOT_FOUND {
				d.Nack(false, false)
			} else {
				d.Ack(false)
			}
		case _, _ = <-quit:
			goto cleanup
		}
	}

cleanup:
	err_ := ci.Channel.Cancel(tag, false)
	if err_ != nil {
		events <- common.NewEventFromError(common.InstT.CONSUMER, err_)
		// should we return here?,
		// we still need to clean the delivery channel...
	}

	// conuming remaining deliveries,
	// and don't ack them. (make them requeue in amqp)
	for cleared := false; cleared == false; {
		select {
		case d, ok := <-dv:
			if !ok {
				cleared = true
				break
			}
			// requeue
			d.Nack(false, true)
		default:
			cleared = true
		}
	}

	// close output channel
	close(tasks)

	return
}
