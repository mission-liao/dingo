package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/task"
	"github.com/stretchr/testify/suite"
)

type DingoMapperTestSuite struct {
	suite.Suite

	_mps            *_mappers
	_invoker        task.Invoker
	_receipts       chan broker.Receipt
	_tasks          chan task.Task
	_countOfMappers int
}

func TestDingoMapperSuite(t *testing.T) {
	suite.Run(t, &DingoMapperTestSuite{
		_receipts:       make(chan broker.Receipt, 5),
		_tasks:          make(chan task.Task, 5),
		_countOfMappers: 3,
		_invoker:        task.NewDefaultInvoker(),
	})
}

func (me *DingoMapperTestSuite) SetupSuite() {
	me._mps = newMappers(me._tasks, me._receipts)

	// allocate 3 mapper routines
	remain, err := me._mps.more(me._countOfMappers)
	me.Equal(0, remain)
	me.Nil(err)
}

func (me *DingoMapperTestSuite) TearDownSuite() {
	me.Nil(me._mps.done())
	close(me._tasks)
	close(me._receipts)
}

//
// test cases
//

func (me *DingoMapperTestSuite) TestParellelMapping() {
	// make sure those mapper routines would be used
	// when one is blocked.

	// the bottleneck of mapper are:
	// - length of receipt channel
	// - count of mapper routines
	count := me._countOfMappers + cap(me._receipts)
	stepIn := make(chan int, count)
	stepOut := make(chan int, count)
	me._mps.allocateWorkers(&StrMatcher{"test"}, func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}, 1)

	// send enough tasks to fill mapper routines & tasks channel
	for i := 0; i < count; i++ {
		// compose corresponding task
		t, err := me._invoker.ComposeTask("test", i)
		me.Nil(err)

		// should not be blocked here
		me._tasks <- t
	}

	// unless worked as expected, or we won't reach
	// this line
	rets := []int{}
	for i := 0; i < count; i++ {

		// consume 1 receipts
		<-me._receipts

		// consume 2 report
		<-me._mps.reports()
		<-me._mps.reports()

		// let 1 worker get out
		stepOut <- 1

		// consume another report
		<-me._mps.reports()

		rets = append(rets, <-stepIn)
	}

	me.Len(rets, count)
}
