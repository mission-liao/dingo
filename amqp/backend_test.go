package dgamqp

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type amqpBackendTestSuite struct {
	dingo.BackendTestSuite
}

func (me *amqpBackendTestSuite) SetupTest() {
	me.BackendTestSuite.Task = nil
}

func (me *amqpBackendTestSuite) TearDownTest() {
	if me.BackendTestSuite.Task == nil {
		return
	}

	// check if corresponding queue is deleted
	func() {
		// get a channel
		isClose := true
		ch, err := me.Bkd.(*backend).Channel()
		me.Nil(err)
		defer func() {
			if isClose {
				me.Bkd.(*backend).ReleaseChannel(ch)
			} else {
				me.Bkd.(*backend).ReleaseChannel(nil)
			}
		}()

		// passive-declare, would be fail if queue doesn't exist
		_, err = ch.Channel.QueueDeclarePassive(
			getQueueName(me.Task),
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

func (me *amqpBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	me.Bkd, err = NewBackend(DefaultAmqpConfig())
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func TestAmqpBackendSuite(t *testing.T) {
	suite.Run(t, &amqpBackendTestSuite{})
}
