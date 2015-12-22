package dgamqp

import (
	// standard
	"errors"
	"fmt"
	"sync"

	// open source
	"github.com/streadway/amqp"

	// internal
	"github.com/mission-liao/dingo"
)

type backend struct {
	sender, receiver *AmqpConnection

	// store
	stores   *dingo.HetroRoutines
	rids     map[string]map[string]int
	ridsLock sync.Mutex

	// reporter
	reporters *dingo.HetroRoutines
	cfg       AmqpConfig
}

func NewBackend(cfg *AmqpConfig) (v *backend, err error) {
	v = &backend{
		reporters: dingo.NewHetroRoutines(),
		rids:      make(map[string]map[string]int),
		cfg:       *cfg,
		stores:    dingo.NewHetroRoutines(),
	}

	v.sender, err = newConnection(&v.cfg)
	if err != nil {
		return
	}

	v.receiver, err = newConnection(&v.cfg)
	if err != nil {
		return
	}

	// define exchange
	ci, err := v.sender.Channel()
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			v.sender.ReleaseChannel(nil)
		} else {
			v.sender.ReleaseChannel(ci)
		}
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

	return
}

//
// dingo.Object interface
//

func (me *backend) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		me.reporters.Events(),
		me.stores.Events(),
	}, nil
}

func (me *backend) Close() (err error) {
	me.reporters.Close()
	me.stores.Close()

	err = me.sender.Close()
	err_ := me.receiver.Close()
	if err == nil {
		err = err_
	}

	return
}

//
// Reporter interface
//

func (me *backend) ReporterHook(eventID int, payload interface{}) (err error) {
	switch eventID {
	case dingo.ReporterEvent.BeforeReport:
		func() {
			ci, err := me.sender.Channel()
			if err != nil {
				return
			}
			defer func() {
				if err != nil {
					me.sender.ReleaseChannel(ci)
				} else {
					me.sender.ReleaseChannel(nil)
				}
			}()

			id := payload.(dingo.Meta)
			qName, rKey := getQueueName(id), getRoutingKey(id)

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
		}()
	}

	return
}

func (me *backend) Report(reports <-chan *dingo.ReportEnvelope) (id int, err error) {
	quit, done, id := me.reporters.New(0)
	go me._reporter_routine_(quit, done, me.reporters.Events(), reports)
	return
}

//
// Store interface
//

func (me *backend) Poll(meta dingo.Meta) (reports <-chan []byte, err error) {
	// bind to the queue for this task
	tag, qName, rKey := getConsumerTag(meta), getQueueName(meta), getRoutingKey(meta)
	quit, done, idx := me.stores.New(0)

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	if v, ok := me.rids[meta.Name()]; ok {
		v[meta.ID()] = idx
	} else {
		me.rids[meta.Name()] = map[string]int{meta.ID(): idx}
	}

	// acquire a free channel
	ci, err := me.receiver.Channel()
	if err != nil {
		return
	}
	var (
		dv <-chan amqp.Delivery
		r  chan []byte
	)
	defer func() {
		if err != nil {
			me.receiver.ReleaseChannel(ci)
		} else {
			go me._store_routine_(quit, done, me.stores.Events(), r, ci, dv, meta)
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

func (me *backend) Done(meta dingo.Meta) (err error) {
	var v int
	err = func() (err error) {
		var (
			ok  bool
			ids map[string]int
		)

		me.ridsLock.Lock()
		defer me.ridsLock.Unlock()

		if ids, ok = me.rids[meta.Name()]; ok {
			v, ok = ids[meta.ID()]
			delete(ids, meta.ID())
		}
		if !ok {
			err = errors.New("store id not found")
			return
		}
		return
	}()
	if err == nil {
		err = me.stores.Stop(v)
	}
	return
}

//
// routine definition
//

func (me *backend) _reporter_routine_(quit <-chan int, done chan<- int, events chan<- *dingo.Event, reports <-chan *dingo.ReportEnvelope) {
	defer func() {
		done <- 1
	}()

	out := func(e *dingo.ReportEnvelope) (err error) {
		// report an error event when leaving
		defer func() {
			if err != nil {
				events <- dingo.NewEventFromError(dingo.ObjT.REPORTER, err)
			}
		}()

		// acquire a channel
		ci, err := me.sender.Channel()
		if err != nil {
			return
		}
		defer me.sender.ReleaseChannel(ci)

		// QueueDeclare and QueueBind should be done in Poll(...)
		err = ci.Channel.Publish(
			"dingo.x.result",    // name of exchange
			getRoutingKey(e.ID), // routing key
			false,               // madatory
			false,               // immediate
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
		cf := <-ci.Confirm
		if !cf.Ack {
			err = errors.New("Unable to publish to server")
			return
		}

		return
	}

finished:
	for {
		select {
		case _, _ = <-quit:
			break finished
		case e, ok := <-reports:
			if !ok {
				// reports channel is closed
				break finished
			}
			out(e)
		}
	}

done:
	// cosume all remaining reports
	for {
		select {
		case e, ok := <-reports:
			if !ok {
				break done
			}
			out(e)
		default:
			break done
		}
	}
}

func (me *backend) _store_routine_(
	quit <-chan int,
	done chan<- int,
	events chan<- *dingo.Event,
	reports chan<- []byte,
	ci *AmqpChannel,
	dv <-chan amqp.Delivery,
	id dingo.Meta) {

	var (
		err            error
		isChannelError bool = false
	)

	defer func() {
		if isChannelError {
			// when error occurs, this channel is
			// automatically closed.
			me.receiver.ReleaseChannel(nil)
		} else {
			me.receiver.ReleaseChannel(ci)
		}
		done <- 1
		close(done)
		close(reports)
	}()

finished:
	for {
		select {
		case _, _ = <-quit:
			break finished
		case d, ok := <-dv:
			if !ok {
				break finished
			}

			d.Ack(false)
			reports <- d.Body
		}
	}

	// consuming remaining stuffs in queue
done:
	for {
		select {
		case d, ok := <-dv:
			if !ok {
				break done
			}

			d.Ack(false)
			reports <- d.Body
		default:
			break done
		}
	}

	// cancel consuming
	err = ci.Channel.Cancel(getConsumerTag(id), false)
	if err != nil {
		events <- dingo.NewEventFromError(dingo.ObjT.STORE, err)
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
		events <- dingo.NewEventFromError(dingo.ObjT.STORE, err)
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
		events <- dingo.NewEventFromError(dingo.ObjT.STORE, err)
		isChannelError = true
		return
	}
}

//
// private function
//

//
func getQueueName(meta dingo.Meta) string {
	return fmt.Sprintf("dingo.q.%s.%s", meta.Name(), meta.ID())
}

//
func getRoutingKey(meta dingo.Meta) string {
	return fmt.Sprintf("dingo.rkey.%s.%s", meta.Name(), meta.ID())
}

//
func getConsumerTag(meta dingo.Meta) string {
	return fmt.Sprintf("dingo.consumer.%s.%s", meta.Name(), meta.ID())
}
