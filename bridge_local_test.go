package dingo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type localBridgeTestSuite struct {
	bridgeTestSuite
}

func (ts *localBridgeTestSuite) SetupTest() {
	ts.bridgeTestSuite.SetupTest()
	ts.Nil(ts.bg.AttachProducer(nil))
	ts.Nil(ts.bg.AttachConsumer(nil, nil))
	ts.Nil(ts.bg.AttachReporter(nil))
	ts.Nil(ts.bg.AttachStore(nil))
}

func TestBridgeLocalSuite(t *testing.T) {
	suite.Run(t, &localBridgeTestSuite{
		bridgeTestSuite{
			name: "local",
		},
	})
}

//
// test cases
//

func (ts *localBridgeTestSuite) TestDuplicatedPolling() {
	var err error
	defer func() {
		ts.Nil(err)
	}()

	ts.Nil(ts.trans.Register("TestDuplicatedPolling", func() {}))

	bg := newLocalBridge()
	defer func() {
		ts.Nil(bg.Close())
	}()

	// listen to event channel
	eventMux := newMux()
	defer eventMux.Close()

	events, err := bg.Events()
	if err != nil {
		return
	}
	_, err = eventMux.Register(events[0], 0)
	if err != nil {
		return
	}
	done := make(chan int, 1)
	eventMux.Handle(func(val interface{}, id int) {
		if e, ok := val.(*Event); ok {
			if EventLvl.Error == e.Level && strings.HasPrefix(e.Payload.(error).Error(), "duplicated polling") {
				done <- 1
			}
		}
	})
	_, err = eventMux.More(1)
	if err != nil {
		return
	}

	// compose a task
	task, err := ts.trans.ComposeTask("TestDuplicatedPolling", nil, nil)
	if err != nil {
		return
	}

	// attach a report channel
	reports := make(chan *Report, 1)
	err = bg.Report(reports)
	if err != nil {
		return
	}

	// send a report
	report, err := task.composeReport(Status.Sent, nil, nil)
	if err != nil {
		return
	}
	reports <- report

	// polling 1st time
	_, err = bg.Poll(task)
	if err != nil {
		return
	}

	_, err = bg.Poll(task)
	if err != nil {
		return
	}

	<-done
}
