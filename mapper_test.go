package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type mapperTestSuite struct {
	suite.Suite

	_mps            *_mappers
	_tasks          chan *transport.Task
	_trans          *transport.Mgr
	_hooks          exHooks
	_countOfMappers int
	_receiptsMux    *common.Mux
	_receipts       chan *TaskReceipt
}

func TestMapperSuite(t *testing.T) {
	suite.Run(t, &mapperTestSuite{
		_trans:          transport.NewMgr(),
		_hooks:          newLocalBridge().(exHooks),
		_tasks:          make(chan *transport.Task, 5),
		_countOfMappers: 3,
		_receiptsMux:    common.NewMux(),
		_receipts:       make(chan *TaskReceipt, 1),
	})
}

func (me *mapperTestSuite) SetupSuite() {
	var err error
	me._mps, err = newMappers(me._trans, me._hooks)
	me.Nil(err)

	// allocate 3 mapper routines
	for remain := me._countOfMappers; remain > 0; remain-- {
		receipts := make(chan *TaskReceipt, 10)
		me._mps.more(me._tasks, receipts)
		_, err := me._receiptsMux.Register(receipts, 0)
		me.Nil(err)
	}
	remain, err := me._receiptsMux.More(3)
	me.Equal(0, remain)
	me.Nil(err)

	me._receiptsMux.Handle(func(val interface{}, _ int) {
		me._receipts <- val.(*TaskReceipt)
	})
}

func (me *mapperTestSuite) TearDownSuite() {
	me.Nil(me._mps.Close())
	close(me._tasks)
	me._receiptsMux.Close()
	close(me._receipts)
}

//
// test cases
//

func (me *mapperTestSuite) TestParellelMapping() {
	// make sure those mapper routines would be used
	// when one is blocked.

	// the bottleneck of mapper are:
	// - length of receipt channel
	// - count of mapper routines
	count := me._countOfMappers + cap(me._tasks)
	stepIn := make(chan int, count)
	stepOut := make(chan int, count)
	fn := func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}
	me.Nil(me._trans.Register(
		"ParellelMapping", fn,
		transport.Encode.Default, transport.Encode.Default, transport.ID.Default,
	))

	reports, remain, err := me._mps.allocateWorkers("ParellelMapping", 1, 0)
	me.Nil(err)
	me.Equal(0, remain)
	me.Len(reports, 1)

	// send enough tasks to fill mapper routines & tasks channel
	for i := 0; i < count; i++ {
		// compose corresponding task
		t, err := me._trans.ComposeTask(
			"ParellelMapping",
			transport.NewOption().SetMonitorProgress(true),
			[]interface{}{i},
		)
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
		<-reports[0]
		<-reports[0]

		// let 1 worker get out
		stepOut <- 1

		// consume another report
		<-reports[0]

		rets = append(rets, <-stepIn)
	}

	me.Len(rets, count)
}
