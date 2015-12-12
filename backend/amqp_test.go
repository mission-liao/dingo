package backend

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AmqpBackendTestSuite struct {
	BackendTestSuite
}

func (me *AmqpBackendTestSuite) SetupTest() {
	me.BackendTestSuite._task = nil
}

func (me *AmqpBackendTestSuite) TearDownTest() {
	if me.BackendTestSuite._task == nil {
		return
	}

	// check if corresponding queue is deleted
	func() {
		// get a channel
		isClose := true
		ch, err := me._backend.(*_amqp).Channel()
		me.Nil(err)
		defer func() {
			if isClose {
				me._backend.(*_amqp).ReleaseChannel(ch)
			} else {
				me._backend.(*_amqp).ReleaseChannel(nil)
			}
		}()

		// passive-declare, would be fail if queue doesn't exist
		_, err = ch.Channel.QueueDeclarePassive(
			getQueueName(me._task),
			true,
			false,
			false,
			false,
			nil,
		)
		me.NotNil(err)
		if err != nil {
			// when an error occurred, this channel would be closed
			// automatically, do not release it.
			isClose = false
		}
	}()
}

func (me *AmqpBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	me._backend, err = New("amqp", cfg)
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func TestAmqpBackendSuite(t *testing.T) {
	suite.Run(t, &AmqpBackendTestSuite{})
}
