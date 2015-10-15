package dingo

import (
	"sort"
	"testing"

	"github.com/mission-liao/dingo/task"
	"github.com/stretchr/testify/suite"
)

type DingoWorkerTestSuite struct {
	suite.Suite

	_ws      *_workers
	_invoker task.Invoker
}

func TestDingoWorkerSuite(t *testing.T) {
	suite.Run(t, &DingoWorkerTestSuite{})
}

func (me *DingoWorkerTestSuite) SetupSuite() {
	me._ws = newWorkers()
	me._invoker = task.NewDefaultInvoker()
}

func (me *DingoWorkerTestSuite) TearDownSuite() {
	me._ws.done()
}

//
// test cases
//

func (me *DingoWorkerTestSuite) TestParellelRun() {
	// make sure other workers would be called
	// when one is blocked.

	stepIn := make(chan int, 3)
	stepOut := make(chan int)

	me._ws.allocate(newStrMatcher("test"), func(i int) {
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

func (me *DingoWorkerTestSuite) TestPanic() {
	// TODO: worker routine should recover from
	// panic
}
