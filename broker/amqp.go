package broker

import (
	// standard
	"encoding/json"
	"errors"
	"fmt"
	"time"

	// open source
	"github.com/streadway/amqp"

	// internal
	"../internal/share"
	"../task"
)

type _control struct {
	quit chan<- int
	done <-chan int
}

type _amqp struct {
	sender, receiver share.Server
	consumerTags     chan int
	controls         map[string]*_control
}

//
// internal/share.Server interface
//

func (me *_amqp) Init() (err error) {
	// init sender, channels are ready after this step
	me.sender = &share.AmqpConn{}
	err = me.sender.Init()
	if err != nil {
		return
	}

	// init receiver, channels are ready after this step
	me.receiver = &share.AmqpConn{}
	err = me.receiver.Init()

	// define relation between queue and exchange
	{
		// get a free channel,
		// either sender/receiver's channel would works
		var ci *share.AmqpChannel
		select {
		case ci <- me.sender.Channels:
			break
		default:
			err = errors.New("No channel available")
			return
		}

		defer func() {
			// remember to return channel to pool
			me.sender.Channels <- ci
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

		// init queue
		err = ci.Channel.QueueDeclare(
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

		// init qos
		err = ci.Channel.Qos(3, 0, true)
		if err != nil {
			return
		}
	}

	// TODO: get number from MaxChannel
	consumerTags = make(chan int, 200)
	for i := 0; i < 200; i++ {
		consumerTags <- i
	}

	me.controls = make(map[string]*_control)

	return
}

func (me *_amqp) Close() (err error) {
	err = me.sender.Close()
	err_ = me.receiver.Close()
	if err == nil && err_ != nil {
		err = err_
	}

	// stop all running 'Receive' go routine
	for _, v := range me.controls {
		v.quit <- 1
		<-v.done

		close(v.quit)
		close(v.done)
	}

	return
}

//
// Sender interface
//

func (me *_amqp) Send(t task.Task) (err error) {
	// marshaling to json
	body, err := json.Marshal(t)
	if err != nil {
		return
	}

	// acquire a channel
	ci := <-me.sender.Channels
	defer func(ci *share.AmqpChannel) {
		// release it when leaving this function
		me.sender.Channels <- ci
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
	case time.After(10 * time.Second):
		// time out
		err = errors.New("Server didn't reply 'Cancel' event in 10 seconds")
	}

	return
}

//
// Receiver interface
//

func (me *_amqp) NewReceiver(tasks chan<- taskInfo, errs chan<- errInfo) (string, error) {
	// acquire an tag
	id := <-me.consumerTags
	tag := fmt.Sprintf("dingo.consumer.%d", id)

	// init channels for control
	ctrl := _control{
		done: make(chan int, 1),
		quit: make(chan int, 1),
	}
	me.controls[tag] = &ctrl

	go func(quit chan<- int, done <-chan int) {
		// return id
		defer func(id int) {
			me.consumerTags <- id
		}(id)

		// acquire a channel
		ci := <-me.receiver.Channels
		defer func(c *share.AmqpChannel) {
			// release it when leaving this function
			me.receiver.Channels <- c
		}(ci)

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
			errs <- errInfo{
				err: err,
				Id:  tag,
			}
			return
		}

		for {
			select {
			case d := <-dv:
				t, err := task.UnmarshalTask(d.Body)
				if err {
					// an invalid task
					d.Nack(false)

					errs <- errInfo{
						err: err,
						Id:  tag,
					}
				} else {
					done := make(chan Receipt, 1) // never blocking
					tasks <- taskInfo{
						T:    t,
						Done: done,
					}

					// raise a go routine for Ack
					go func(d *amqp.Delivery, done chan Receipt) {
						reply := <-done
						close(done)

						if reply.Status == Status.WORKER_NOT_FOUND {
							d.Nack(false)
						} else {
							d.Ack(false)
						}
					}(&d, done)
				}
			case <-quit:
				err := ci.Channel.Cancel(tag, false)
				if err != nil {
					errs <- errInfo{
						err: err,
						Id:  tag,
					}
					// do not return here,
					// we need to clean the delivery channel
				}

				// wait for reply from server for cancel event
				select {
				case name := <-ci.Cancel:
					break
				case <-time.After(10 * time.Second): // TODO: configuration
					errs <- errors.New("Server didn't reply 'Cancel' event in 10 seconds")
				}

				// conuming remaining deliveries,
				// and don't ack them. (make them requeue in amqp)
				for cleared := false; cleared == false; {
					select {
					case <-d:
						break
					default:
						cleared = true
					}
				}

				finished <- 1
			}
		}
	}(ctrl.quit, ctrl.done)

	return tag, nil
}
