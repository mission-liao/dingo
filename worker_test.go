package dingo

import (
	"sort"
	"testing"

	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type WorkerTestSuite struct {
	suite.Suite

	_ws    *_workers
	_trans *transport.Mgr
}

func TestWorkerSuite(t *testing.T) {
	suite.Run(t, &WorkerTestSuite{})
}

func (me *WorkerTestSuite) SetupSuite() {
	var err error
	me._trans = transport.NewMgr()
	me._ws, err = newWorkers(me._trans)
	me.Nil(err)
}

func (me *WorkerTestSuite) TearDownSuite() {
	me.Nil(me._ws.Close())
}

//
// test cases
//

func (me *WorkerTestSuite) TestParellelRun() {
	// make sure other workers would be called
	// when one is blocked.

	stepIn := make(chan int, 3)
	stepOut := make(chan int)
	tasks := make(chan *transport.Task)
	fn := func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}
	me._trans.Register(
		"", fn,
		transport.Encode.Default, transport.Encode.Default,
	)
	reports, remain, err := me._ws.allocate("", tasks, nil, 3, 0)
	me.Nil(err)
	me.Equal(0, remain)
	me.Len(reports, 1)

	for i := 0; i < 3; i++ {
		t, err := transport.ComposeTask("", []interface{}{i})
		me.Nil(err)
		if err == nil {
			tasks <- t
		}
	}

	rets := []int{}
	for i := 0; i < 3; i++ {
		rets = append(rets, <-stepIn)
	}
	sort.Ints(rets)
	me.Equal([]int{0, 1, 2}, rets)

	stepOut <- 1
	stepOut <- 1
	stepOut <- 1
	close(stepIn)
	close(stepOut)
}

func (me *WorkerTestSuite) TestPanic() {
	// TODO: worker routine should recover from
	// panic
}
