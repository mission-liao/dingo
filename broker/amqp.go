package broker

import (
	// standard
	"encoding/json"
	"errors"
	"fmt"

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
// Publisher interface
//

func (me *_amqp) Send(t task.Task) (err error) {
	// marshaling to json
	body, err := json.Marshal(t)
	if err != nil {
		return
	}

	// acquire a channel
	ci := <-me.Channels
	defer func() {
		// release it when leaving this function
		me.Channels <- ci
	}()

	err = ci.Channel.Publish(
		"dingo.default", // name of exchange
		"",              // routing key
		false,           // mandatory
		false,           // immediate
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
	// TODO: time out
	cf := <-ci.Confirm
	if !cf.Ack {
		err = errors.New("Unable to publish to server")
	}

	return
}

//
// Consumer interface
//

func (me *_amqp) Recv(w task.Worker) error {
	// TODO:
}
