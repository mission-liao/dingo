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
// consumer
//

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

	conn := cfg.AMQP_().Connection()
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
// common.Server interface
//

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

func (me *_amqp) AddListener(receipts <-chan Receipt) (<-chan meta.Task, <-chan error, error) {
	tasks := make(chan meta.Task, 10)
	errs := make(chan error, 10)
	quit, wait := me.consumers.New()
	go me._consumer_routine_(quit, wait, tasks, errs, receipts)

	return tasks, errs, nil
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
	tasks chan<- meta.Task,
	errs chan<- error,
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
		errs <- err
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
		errs <- err
		return
	}

	for {
		select {
		case d, _ := <-dv:
			t, err_ := meta.UnmarshalTask(d.Body)
			if err_ != nil {
				// an invalid task
				d.Nack(false, false)
				errs <- err_
				break
			}

			tasks <- t
			reply, ok := <-receipts
			if ok {
				if reply.Status == Status.WORKER_NOT_FOUND {
					d.Nack(false, false)
				} else {
					d.Ack(false)
				}
			} else {
				goto cleanup
			}
		case <-quit:
			goto cleanup
		}
	}

cleanup:
	err_ := ci.Channel.Cancel(tag, false)
	if err_ != nil {
		errs <- err_
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
	close(errs)

	return
}
