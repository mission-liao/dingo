package common

//
// 'routines' helps to manage go-routines
// depending on the same input/output channels
//

import (
	"sync"
)

type Routines struct {
	quits []chan int
	qLock sync.Mutex
	wg    sync.WaitGroup
}

func NewRoutines() *Routines {
	return &Routines{
		quits: make([]chan int, 0, 10),
	}
}

func (me *Routines) New() (<-chan int, *sync.WaitGroup) {
	me.wg.Add(1)

	me.qLock.Lock()
	defer me.qLock.Unlock()

	me.quits = append(me.quits, make(chan int, 1))
	return me.quits[len(me.quits)-1], &me.wg
}

func (me *Routines) Close() {
	me.qLock.Lock()
	defer me.qLock.Unlock()

	for _, v := range me.quits {
		v <- 1
		close(v)
	}
	me.quits = make([]chan int, 0, 10)
	me.wg.Wait()
}
