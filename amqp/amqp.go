package dgamqp

import (
	// standard
	"errors"
	"sync"

	// open source
	"github.com/streadway/amqp"
)

//
// wrapper of amqp.Channel
//

type AmqpChannel struct {
	Channel *amqp.Channel
	Confirm chan amqp.Confirmation
	Cancel  chan string
}

//
// wrapper of amqp.Connection
//

type AmqpConnection struct {
	conn *amqp.Connection

	// watch dogs
	cLock                   sync.Mutex
	channels                chan *AmqpChannel // channel pool
	cntChannels, maxChannel int
	noMore                  bool // we already created maxChannels
}

func newConnection(cfg *AmqpConfig) (c *AmqpConnection, err error) {
	c = &AmqpConnection{
		maxChannel:  cfg.GetMaxChannel(),
		cntChannels: 0,
	}

	c.conn, err = amqp.Dial(cfg.Connection())
	if err != nil {
		return
	}

	c.channels = make(chan *AmqpChannel, c.maxChannel)
	initCount := 16
	for ; initCount > 0; initCount-- {
		var ch *AmqpChannel
		ch, err = c.Channel()
		if err != nil {
			return
		}
		c.ReleaseChannel(ch)
	}

	return
}

func (me *AmqpConnection) Close() error {
	var err error

	// stop allocating new channels
	me.noMore = true

	func() {
		me.cLock.Lock()
		defer me.cLock.Unlock()

		for remain := me.cntChannels; remain > 0; remain-- {
			// we may dead in this line
			select {
			case ci := <-me.channels:
				// TODO: log errors
				// close it
				err_ := ci.Channel.Close()
				if err == nil {
					err = err_
				}
			}
		}

		if me.channels != nil {
			close(me.channels)
		}
	}()

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
// helper function
//

func (me *AmqpConnection) Channel() (ch *AmqpChannel, err error) {
	ok := true

	select {
	case ch, ok = <-me.channels:
		break
	default:
		// with this,
		// we can skip checking of mutex.
		if me.noMore {
			ch, ok = <-me.channels
			break
		}

		me.cLock.Lock()
		defer me.cLock.Unlock()

		if me.cntChannels < me.maxChannel && !me.noMore {
			var c *amqp.Channel
			c, err = me.conn.Channel()

			// enable publisher confirm
			err = c.Confirm(false)
			if err != nil {
				break
			}

			ch = &AmqpChannel{
				Channel: c,
				Confirm: c.NotifyPublish(make(chan amqp.Confirmation, 1)),
				Cancel:  c.NotifyCancel(make(chan string, 1)),
			}
			me.cntChannels++

		} else {
			me.noMore = true
			ch, ok = <-me.channels
			break
		}
	}

	if !ok {
		err = errors.New("channel pool is closed")
	}

	return
}

func (me *AmqpConnection) ReleaseChannel(ci *AmqpChannel) {
	if ci != nil {
		me.channels <- ci
	} else {
		func() {
			me.cLock.Lock()
			defer me.cLock.Unlock()

			me.noMore = false
			me.cntChannels--
		}()
	}
}
