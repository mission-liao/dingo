package backend

import (
	// standard
	"encoding/json"
	"fmt"

	// open source
	"github.com/streadway/amqp"

	// internal
	"../internal/share"
	"../task"
)

type _amqp struct {
	share.AmqpConn
}

//
// internal/share.Server interface
//

func (me *_amqp) Init() (err error) {
	// call parent's Init
	err = me.AmqpConn.Init()
	if err != nil {
		return
	}

	// define exchange
	{
		// get a free channel
		var ci *share.AmqpChannel
		select {
		case ci <- me.AmqpConn.Channels:
			break
		default:
			err = errors.New("No channel available")
			return
		}

		defer func() {
			me.AmqpConn.Channels <- ci
		}()

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
	}
}

//
// Backend interface
//

func (me *_amqp) Update(r task.Report) (err error) {
	// marshalling to json
	body, err := json.Marshal(r)

	// acquire a channel
	ci := <-me.AmqpConn.Channels
	defer func() {
		me.AmqpConn.Channels <- ci
	}()

	qName, rKey := getQueueName(r), getRoutingKey(r)

	// declare a queue for this task
	err = ci.Channel.QueueDeclare(
		getQueueName(), // name of queue
		true,           // durable
		false,          // auto-delete
		false,          // exclusive
		false,          // noWait
		nil,            // args
	)
	if err != nil {
		return
	}

	// bind queue to result-exchange
	err = ci.Channel.QueueBind(
		qName,            // name of queue
		rKey,             // routing key
		"dingo.x.result", // name of exchange
		false,            // noWait
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
		amqp.Publish{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/json",
			Body:         body,
		},
	)
	if err != nil {
		return
	}

	// block until amqp.Channel.NotifyPublish
	// TODO: time out
	cf := <-ci.Confirm
	if !cf.Ack {
		err = errors.New("Unable to publish to server")
	}

	return
}

func (me *_amqp) NewPoller(report chan<- task.Report) (err error) {
	// TODO: totally rewrite
}

func (me *_amqp) Check(t task.Task) (err error) {
}

func (me *_amqp) Uncheck(r task.Report) (err error) {
}

//
// private function
//

//
func getQueueName(r task.Report) string {
	return fmt.Sprintf("dingo.q.%q", r.GetId())
}

//
func getRoutingKey(r task.Report) string {
	return fmt.Sprintf("dingo.rkey.%q", r.GetId())
}
