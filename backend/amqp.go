package backend

import (
	// standard
	"encoding/json"

	// open source
	"github.com/streadway/amqp"

	// internal
	"../internal/share"
	"../task"
)

type _amqp struct {
	share.AmqpHelper
}

//
// Backend interface
//

func (me *_amqp) Update(r task.Report) (err error) {
	// marshalling to json
	body, err := json.Marshal(r)

	// acquire a channel
	ci := <-me.Channels
	defer func() {
		me.Channels <- ci
	}()

	// TODO: different exchange, queue for backend/broker
	err = ci.Channel.Publish(
		"dingo.default", // name of exchange
		"",              // routing key
		false,           // madatory
		false,           // immediate
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

func (me *_amqp) Poll(t task.Task, last task.Report) (task.Report, error) {
}
