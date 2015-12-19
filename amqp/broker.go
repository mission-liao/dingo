package dgamqp

import (
	// standard
	"errors"
	"fmt"
	"sync"

	// open source
	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/streadway/amqp"
)

//
// major component
//

type broker struct {
	sender, receiver *AmqpConnection
	consumerTags     chan int
	consumers        *common.Routines
	cfg              AmqpConfig
}

func NewBroker(cfg *AmqpConfig) (v *broker, err error) {
	v = &broker{
		consumers: common.NewRoutines(),
		cfg:       *cfg,
	}

	v.sender, err = newConnection(&v.cfg)
	if err != nil {
		return
	}

	v.receiver, err = newConnection(&v.cfg)
	if err != nil {
		return
	}

	// get a free channel,
	// either sender/receiver's channel would works
	ci, err := v.sender.Channel()
	if err != nil {
		return
	}

	// remember to return channel to pool
	defer func() {
		if err != nil {
			v.sender.ReleaseChannel(nil)
		} else {
			v.sender.ReleaseChannel(ci)
		}
	}()

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

	v.consumerTags = make(chan int, v.receiver.maxChannel)
	for i := 0; i < v.receiver.maxChannel; i++ {
		v.consumerTags <- i
	}

	return
}

//
// common.Object interface
//

func (me *broker) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.consumers.Events(),
	}, nil
}

func (me *broker) Close() (err error) {
	me.consumers.Close()

	err = me.sender.Close()
	err_ := me.receiver.Close()
	if err == nil {
		// error from sender is propagated first
		err = err_
	}

	return
}

//
// Producer interface
//

func (me *broker) ProducerHook(eventID int, payload interface{}) (err error) {
	switch eventID {
	case dingo.ProducerEvent.DeclareTask:
		func() {
			ci, err := me.sender.Channel()
			if err != nil {
				return
			}
			// remember to return channel to pool
			defer func() {
				if err == nil {
					me.sender.ReleaseChannel(ci)
				} else {
					me.sender.ReleaseChannel(nil)
				}
			}()

			queueName := getConsumerQueueName(name)

			// init queue
			_, err = ci.Channel.QueueDeclare(
				queueName, // name of queue
				true,      // durable
				false,     // auto-delete
				false,     // exclusive
				false,     // noWait
				nil,       // args
			)
			if err != nil {
				return
			}

			// bind queue to exchange
			err = ci.Channel.QueueBind(
				queueName,
				name,
				"dingo.x.task",
				false, // noWait
				nil,   // args
			)
			if err != nil {
				return
			}

		}()
	}
	return
}

func (me *broker) Send(id transport.Meta, body []byte) (err error) {
	// acquire a channel
	ci, err := me.sender.Channel()
	if err != nil {
		return
	}
	defer me.sender.ReleaseChannel(ci)

	err = ci.Channel.Publish(
		"dingo.x.task", // name of exchange
		id.Name(),      // routing key
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
	case cf, ok := <-ci.Confirm:
		if !ok {
			err = errors.New(fmt.Sprintf("confirm channel is closed before receiving confirm: %v", id))
		}
		if !cf.Ack {
			err = errors.New(fmt.Sprintf("Unable to publish to server: %v", id))
		}
	}

	return
}

//
// Consumer interface
//

func (me *broker) AddListener(name string, receipts <-chan *dingo.TaskReceipt) (tasks <-chan []byte, err error) {
	ci, err := me.receiver.Channel()
	if err != nil {
		return
	}
	qn := getConsumerQueueName(name)

	// remember to return channel to pool
	defer func() {
		if err != nil {
			me.receiver.ReleaseChannel(ci)
		} else {
			t := make(chan []byte, 10)
			go me._consumer_routine_(me.consumers.New(), me.consumers.Wait(), me.consumers.Events(), t, receipts, ci, qn)

			tasks = t
		}
	}()

	// init queue
	_, err = ci.Channel.QueueDeclare(
		qn,    // name of queue
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return
	}

	// bind queue to exchange
	err = ci.Channel.QueueBind(
		qn,
		name,
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

	return
}

func (me *broker) StopAllListeners() (err error) {
	me.consumers.Close()
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
	ci *AmqpChannel,
	queueName string,
) {
	defer wait.Done()
	defer me.receiver.ReleaseChannel(ci)

	// acquire an tag
	id := <-me.consumerTags
	tag := fmt.Sprintf("dingo.consumer.%d", id)

	// return id
	defer func(id int) {
		me.consumerTags <- id
	}(id)

	dv, err := ci.Channel.Consume(
		queueName,
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
				goto clean
			}

			ok = func() (ok bool) {
				var (
					reply *dingo.TaskReceipt
					err   error
				)
				defer func() {
					if err != nil || !ok {
						d.Nack(false, false)
						events <- common.NewEventFromError(common.InstT.CONSUMER, err)
					} else {
						d.Ack(false)
					}
				}()
				h, err := transport.DecodeHeader(d.Body)
				if err != nil {
					return
				}
				tasks <- d.Body
				// block here for receipts
				reply, ok = <-receipts
				if !ok {
					return
				}

				if reply.ID != h.ID() {
					err = errors.New(fmt.Sprintf("expected: %v, received: %v", h, reply))
					return
				}
				if reply.Status == dingo.ReceiptStatus.WORKER_NOT_FOUND {
					err = errors.New(fmt.Sprintf("worker not found: %v", h))
					return
				}
				return
			}()
			if !ok {
				goto clean
			}
		case _, _ = <-quit:
			goto clean
		}
	}

clean:
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

func getConsumerQueueName(name string) string {
	return fmt.Sprintf("dingo.q.task.%v", name)
}
