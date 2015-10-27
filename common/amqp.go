package common

import (
	// standard
	"errors"
	"fmt"
	"sync"

	// open source
	"github.com/streadway/amqp"
)

//
// configuration
//

type AmqpConfig struct {
	Host_     string `json:"Host"`
	Port_     int    `json:"Port"`
	User_     string `json:"User"`
	Password_ string `json:"Password"`
}

func DefaultAmqpConfig() *AmqpConfig {
	return &AmqpConfig{
		Host_:     "localhost",
		Port_:     5672,
		User_:     "guest",
		Password_: "guest",
	}
}

//
// setter
//

func (me *AmqpConfig) Host(host string) *AmqpConfig {
	me.Host_ = host
	return me
}

func (me *AmqpConfig) Port(port int) *AmqpConfig {
	me.Port_ = port
	return me
}

func (me *AmqpConfig) User(user string) *AmqpConfig {
	me.User_ = user
	return me
}

func (me *AmqpConfig) Password(password string) *AmqpConfig {
	me.Password_ = password
	return me
}

//
// getter
//

func (me *AmqpConfig) Connection() string {
	return fmt.Sprintf("amqp://%v:%v@%v:%d/", me.User_, me.Password_, me.Host_, me.Port_)
}

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
	cLock       sync.Mutex
	channels    chan *AmqpChannel // channel pool
	maxChannel  int
	cntChannels int
	noMore      bool // we already created maxChannels
}

//
// common.Server interface
//

func (me *AmqpConnection) Init(conn string) (err error) {
	me.maxChannel = 1024 // TODO: configurable
	me.cntChannels = 0

	// connect to AMQP
	me.conn, err = amqp.Dial(conn)
	if err != nil {
		return
	}

	// create channels
	me.channels = make(chan *AmqpChannel, me.maxChannel)

	initCount := 16
	for ; initCount > 0; initCount-- {
		var ch *AmqpChannel
		ch, err = me.Channel()
		if err != nil {
			return
		}
		me.ReleaseChannel(ch)
	}

	return
}

func (me *AmqpConnection) Close() error {
	var err error

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

	{
		me.cLock.Lock()
		defer me.cLock.Unlock()

		me.noMore = false

		if me.channels != nil {
			close(me.channels)
			me.channels = nil
		}

		if me.conn != nil {
			err_ := me.conn.Close()
			if err_ != nil && err == nil {
				err = err_
			}

			me.conn = nil
		}
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
	}
}
