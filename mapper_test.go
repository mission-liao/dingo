package dingo

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type mapperTestSuite struct {
	suite.Suite

	_mps            *_mappers
	_tasks          chan *Task
	_trans          *fnMgr
	_hooks          exHooks
	_countOfMappers int
	_receiptsMux    *mux
	_receipts       chan *TaskReceipt
}

func TestMapperSuite(t *testing.T) {
	suite.Run(t, &mapperTestSuite{
		_trans:          newFnMgr(),
		_hooks:          newLocalBridge().(exHooks),
		_tasks:          make(chan *Task, 5),
		_countOfMappers: 3,
		_receiptsMux:    newMux(),
		_receipts:       make(chan *TaskReceipt, 1),
	})
}

func (ts *mapperTestSuite) SetupSuite() {
	var err error
	ts._mps, err = newMappers(ts._trans, ts._hooks)
	ts.Nil(err)

	// allocate 3 mapper routines
	for remain := ts._countOfMappers; remain > 0; remain-- {
		receipts := make(chan *TaskReceipt, 10)
		ts._mps.more(ts._tasks, receipts)
		_, err := ts._receiptsMux.Register(receipts, 0)
		ts.Nil(err)
	}
	remain, err := ts._receiptsMux.More(3)
	ts.Equal(0, remain)
	ts.Nil(err)

	ts._receiptsMux.Handle(func(val interface{}, _ int) {
		ts._receipts <- val.(*TaskReceipt)
	})
}

func (ts *mapperTestSuite) TearDownSuite() {
	ts.Nil(ts._mps.Close())
	close(ts._tasks)
	ts._receiptsMux.Close()
	close(ts._receipts)
}

//
// test cases
//

func (ts *mapperTestSuite) TestParellelMapping() {
	// make sure those mapper routines would be used
	// when one is blocked.

	// the bottleneck of mapper are:
	// - length of receipt channel
	// - count of mapper routines
	count := ts._countOfMappers + cap(ts._tasks)
	stepIn := make(chan int, count)
	stepOut := make(chan int, count)
	fn := func(i int) {
		stepIn <- i
		// workers would be blocked here
		<-stepOut
	}
	ts.Nil(ts._trans.Register(
		"ParellelMapping", fn,
	))

	reports, remain, err := ts._mps.allocateWorkers("ParellelMapping", 1, 0)
	ts.Nil(err)
	ts.Equal(0, remain)
	ts.Len(reports, 1)

	// send enough tasks to fill mapper routines & tasks channel
	for i := 0; i < count; i++ {
		// compose corresponding task
		t, err := ts._trans.ComposeTask(
			"ParellelMapping",
			DefaultOption().MonitorProgress(true),
			[]interface{}{i},
		)
		ts.Nil(err)

		// should not be blocked here
		ts._tasks <- t
	}

	// unless worked as expected, or we won't reach
	// this line
	rets := []int{}
	for i := 0; i < count; i++ {
		// consume 1 receipts
		<-ts._receipts

		// consume 2 report
		<-reports[0]
		<-reports[0]

		// let 1 worker get out
		stepOut <- 1

		// consume another report
		<-reports[0]

		rets = append(rets, <-stepIn)
	}

	ts.Len(rets, count)
}

func (ts *mapperTestSuite) TestWorkerNotFound() {
	// make sure worker not found would be raised
	ts.Nil(ts._trans.Register("WorkerNotFound", func() {}))

	t, err := ts._trans.ComposeTask("WorkerNotFound", nil, nil)
	ts.Nil(err)

	ts._tasks <- t
	r := <-ts._receipts
	ts.Equal(t.ID(), r.ID)
	ts.Equal(ReceiptStatus.WorkerNotFound, r.Status)
}
