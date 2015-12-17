package dgamqp

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

type amqpBrokerTestSuite struct {
	dingo.BrokerTestSuite
}

func (me *amqpBrokerTestSuite) TearDownTest() {
	// delete queue
	for _, v := range me.BrokerTestSuite.ConsumerNames {
		func() {
			ch, err := me.Ncsm.(*broker).receiver.Channel()
			me.Nil(err)
			defer func() {
				if err == nil {
					me.Ncsm.(*broker).receiver.ReleaseChannel(ch)
				} else {
					me.Ncsm.(*broker).receiver.ReleaseChannel(nil)
				}
			}()

			_, err = ch.Channel.QueueDelete(
				getConsumerQueueName(v), // name
				false, // ifUnused
				false, // ifEmpty
				false, // noWait
			)
		}()
	}
	me.BrokerTestSuite.TearDownTest()
}

func TestAmqpBrokerSuite(t *testing.T) {
	suite.Run(t, &amqpBrokerTestSuite{
		dingo.BrokerTestSuite{
			Gen: func() (b interface{}, err error) {
				b, err = NewBroker(DefaultAmqpConfig())
				return
			},
		},
	})
}
