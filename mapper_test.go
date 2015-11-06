package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
	"github.com/stretchr/testify/suite"
)

type MapperTestSuite struct {
	suite.Suite

	_mps            *_mappers
	_invoker        meta.Invoker
	_tasks          chan meta.Task
	_countOfMappers int
	_receiptsMux    *common.Mux
}

func TestMapperSuite(t *testing.T) {
	suite.Run(t, &MapperTestSuite{
		_tasks:          make(chan meta.Task, 5),
		_countOfMappers: 3,
		_invoker:        meta.NewDefaultInvoker(),
		_receiptsMux:    common.NewMux(),
	})
}

func (me *MapperTestSuite) SetupSuite() {
	var err error
	me._mps, err = newMappers()
	me.Nil(err)

	// allocate 3 mapper routines
	for remain := me._countOfMappers; remain > 0; remain-- {
		receipts := make(chan broker.Receipt, 10)
		me._mps.more(me._tasks, receipts)
		_, err := me._receiptsMux.Register(receipts, 0)
		me.Nil(err)
	}
	remain, err := me._receiptsMux.More(3)
	me.Equal(0, remain)
	me.Nil(err)
}

func (me *MapperTestSuite) TearDownSuite() {
	me.Nil(me._mps.Close())
	close(me._tasks)
	me._receiptsMux.Close()
}

//
// test cases
//

func (me *MapperTestSuite) TestParellelMapping() {
	// make sure those mapper routines would be used
	// when one is blocked.

	// the bottleneck of mapper are:
	// - length of receipt channel
	// - count of mapper routines
	count := me._countOfMappers + cap(me._tasks)
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
		<-me._receiptsMux.Out()

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
