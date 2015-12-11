package broker

import (
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type BrokerTestSuite struct {
	suite.Suite

	_producer      Producer
	_consumer      Consumer
	_namedConsumer NamedConsumer
}

func (me *BrokerTestSuite) SetupSuite() {
}

func (me *BrokerTestSuite) TearDownSuite() {
	me.Nil(me._producer.(common.Object).Close())
}

func (me *BrokerTestSuite) AddListener(name string, receipts <-chan *Receipt) (tasks <-chan []byte, err error) {
	if me._consumer != nil {
		tasks, err = me._consumer.AddListener(receipts)
	} else if me._namedConsumer != nil {
		tasks, err = me._namedConsumer.AddListener("", receipts)
	}

	return
}

func (me *BrokerTestSuite) StopAllListeners() (err error) {
	if me._consumer != nil {
		err = me._consumer.StopAllListeners()
	} else if me._namedConsumer != nil {
		err = me._namedConsumer.StopAllListeners()
	}

	return
}

//
// test cases
//

func (me *BrokerTestSuite) TestBasic() {
	var (
		tasks <-chan []byte
	)
	// init one listener
	receipts := make(chan *Receipt, 10)
	tasks, err := me.AddListener("", receipts)
	me.Nil(err)
	me.NotNil(tasks)
	if tasks == nil {
		return
	}

	// compose a task
	t, err := transport.ComposeTask("", nil, []interface{}{})
	me.Nil(err)
	me.NotNil(t)
	if t == nil {
		return
	}

	// generate a header byte stream
	hb, err := transport.NewHeader(t.ID(), t.Name()).Flush(0)
	me.Nil(err)
	if err != nil {
		return
	}

	// send it
	input := append(hb, []byte("test byte array")...)
	me.Nil(me._producer.Send(t, input))

	// receive it
	output := <-tasks
	me.Equal(string(input), string(output))

	// send a receipt
	receipts <- &Receipt{
		ID:     t.ID(),
		Status: Status.OK,
	}

	// stop all listener
	me.Nil(me.StopAllListeners())
}
