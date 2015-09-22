package share

import (
	"github.com/streadway/amqp"
)

type _channelInfo struct {
	Channel *amqp.Channel
	Confirm chan amqp.Confirmation
}

type AmqpHelper struct {
	Conn     *amqp.Connection
	Channels chan *_channelInfo // channel pool
}

//
// Broker interface
//

func (me *AmqpHelper) Init() (err error) {
	maxChannel := 16 // TODO: configurable

	// connect to AMQP
	me.conn, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		return
	}

	// create channel
	err = me.prepareChannels(maxChannel)
	if err != nil {
		return
	}

	// get a free channel
	var ci *_channelInfo
	select {
	case ci <- me.channels:
	default:
		err = errors.New("No channel available")
		return
	}

	defer func() {
		// remember to return channel to pool
		me.channels <- ci
	}()

	// init exchange
	err = ci.channel.ExchangeDeclare(
		"dingo.ex.default", // name of exchange
		"direct",           // kind
		true,               // durable
		false,              // auto-delete
		false,              // internal
		false,              // noWait
		nil,                // args
	)
	if err != nil {
		return
	}

	// init queue
	err = ci.channel.QueueDeclare(
		"dingo.q.default", // name of queue
		true,              // durable
		false,             // auto-delete
		false,             // exclusive
		false,             // noWait
		nil,               // args
	)
	if err != nil {
		return
	}

	// bind queue to exchange
	// TODO: configurable name?
	err = ci.channel.QueueBind(
		"dingo.q.default",
		"",
		"dingo.ex.default",
		false, // noWait
		nil,   // args
	)

	// init qos
	err = ci.channel.Qos(3, 0, true)
	if err != nil {
		return
	}

	return
}

func (me *AmqpHelper) Uninit() error {
	var err error

	for releaseCount := 0; releaseCount < cap(me.channels); releaseCount++ {
		// we may dead in this line
		ci := <-me.channels

		// close it
		err_ = ci.channel.Close()
		if err == nil {
			err = err_
		}

		// release chan
		close(ci.confirm)
	}

	close(me.channels)
	me.channels = nil

	if me.conn != nil {
		err_ := me.conn.Close()
		if err_ != nil && err == nil {
			err = err_
		}

		me.conn = nil
	}

	return err
}

//
// private function
//

func (me *AmqpHelper) prepareChannels(maxChannel uint) error {
	if me.conn == nil {
		return errors.New("Connection to AMQP is not made")
	}

	me.channels = make(chan *_channelInfo, maxChannel)

	for ; maxChannel > 0; maxChannel-- {
		ch, err = me.conn.Channel()

		// enable publisher confirm
		err = ch.Confirm(false)
		if err != nil {
			return err
		}

		me.channels <- &_channelInfo{
			channel: ch,
			confirm: ch.NotifyPublish(make(chan amqp.Confirmation)),
		}
	}
	return
}
