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
}

func defaultAmqpConfig() *_amqpConfig {
	return &_amqpConfig{
		AmqpConfig: *common.DefaultAmqpConfig(),
	}
}

type _amqp struct {
	common.AmqpConnection

	// store
	reports  chan meta.Report
	stores   *common.HetroRoutines
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
	err = me.AmqpConnection.Close()
	err_ := me.Unbind()
	if err == nil {
		err = err_
	}
	me.stores.Close()
	return
}

//
// Reporter interface
//

func (me *_amqp) Report(reports <-chan meta.Report) (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	// close previous reporters
	me.reporters.Close()

	remain := me.cfg.Reporters_
	for ; remain > 0; remain-- {
		quit := me.reporters.New()
		go me._reporter_routine_(quit, me.reporters.Wait(), me.reporters.Events(), reports)
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
	quit, done, idx := me.stores.New(0)

	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()
	me.rids[id.GetId()] = idx

	// acquire a free channel
	ci, err := me.AmqpConnection.Channel()
	if err != nil {
		return
	}
	var dv <-chan amqp.Delivery
	defer func() {
		if err != nil {
			me.AmqpConnection.ReleaseChannel(ci)
		} else {
			go me._store_routine_(quit, done, me.stores.Events(), me.reports, ci, dv, id)
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

	return
}

func (me *_amqp) Done(id meta.ID) (err error) {
	me.ridsLock.Lock()
	defer me.ridsLock.Unlock()

	v, ok := me.rids[id.GetId()]
	if !ok {
		err = errors.New("store id not found")
		return
	}

	return me.stores.Stop(v)
}

//
// routine definition
//

func (me *_amqp) _reporter_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event, reports <-chan meta.Report) {
	// TODO: keep one AmqpConnection, instead of get/realease for each report.
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
			body, err := json.Marshal(r)
			if err != nil {
				events <- common.NewEventFromError(common.InstT.REPORTER, err)
				break
			}

			// acquire a channel
			err = func() (err error) {
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

cleanup:
	// TODO: cosume all remaining reports?
}

func (me *_amqp) _store_routine_(
	quit <-chan int,
	done chan<- int,
	events chan<- *common.Event,
	reports chan<- meta.Report,
	ci *common.AmqpChannel,
	dv <-chan amqp.Delivery,
	id meta.ID) {

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
				events <- common.NewEventFromError(common.InstT.STORE, err)
				break
			}

			reports <- r
		}
	}

cleanup:
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
