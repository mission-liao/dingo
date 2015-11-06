package dingo

import (
	"sort"
	"testing"

	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/suite"
)

type WorkerTestSuite struct {
	suite.Suite

	_ws      *_workers
	_invoker meta.Invoker
}

func TestWorkerSuite(t *testing.T) {
	suite.Run(t, &WorkerTestSuite{})
}

func (me *WorkerTestSuite) SetupSuite() {
	var err error
	me._ws, err = newWorkers()
	me.Nil(err)
	me._invoker = meta.NewDefaultInvoker()
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

	me._ws.allocate(&StrMatcher{"test"}, func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}, 3)

	for i := 0; i < 3; i++ {
		t, err := me._invoker.ComposeTask("test", i)
		me.Nil(err)
		if err == nil {
			me.Nil(me._ws.dispatch(t))
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
