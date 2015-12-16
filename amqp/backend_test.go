package dgamqp

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type amqpBackendTestSuite struct {
	dingo.BackendTestSuite
}

func (me *amqpBackendTestSuite) TearDownTest() {
	if len(me.BackendTestSuite.Tasks) == 0 {
		return
	}

	// check if corresponding queue is deleted
	for _, v := range me.BackendTestSuite.Tasks {
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
				getQueueName(v),
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

	me.BackendTestSuite.TearDownTest()
}

func TestAmqpBackendSuite(t *testing.T) {
	suite.Run(t, &amqpBackendTestSuite{
		dingo.BackendTestSuite{
			Gen: func() (b dingo.Backend, err error) {
				b, err = NewBackend(DefaultAmqpConfig())
				return
			},
		},
	})
}
