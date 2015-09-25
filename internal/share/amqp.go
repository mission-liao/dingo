package share

import (
	"github.com/streadway/amqp"
)

type AmqpChannel struct {
	Channel *amqp.Channel
	Confirm chan amqp.Confirmation
}

type AmqpConn struct {
	Conn     *amqp.Connection
	Channels chan *AmqpChannel // channel pool
}

//
// Broker interface
//

func (me *AmqpConn) Init() (err error) {
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

	return
}

func (me *AmqpConn) Close() error {
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

func (me *AmqpConn) prepareChannels(maxChannel uint) error {
	if me.conn == nil {
		return errors.New("Connection to AMQP is not made")
	}

	me.channels = make(chan *AmqpChannel, maxChannel)

	for ; maxChannel > 0; maxChannel-- {
		ch, err = me.conn.Channel()

		// enable publisher confirm
		err = ch.Confirm(false)
		if err != nil {
			return err
		}

		me.channels <- &AmqpChannel{
			channel: ch,
			confirm: ch.NotifyPublish(make(chan amqp.Confirmation, 1)),
			cancel:  ch.NotifyCancel(make(chan string, 1)),
		}
	}
	return
}
