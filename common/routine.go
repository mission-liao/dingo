package common

//
// 'routines' helps to manage go-routines
// depending on the same input/output channels
//

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Homogeneous Routines
//
//

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

// Heterogeneous Routines
//
//

type _control struct {
	quit, done chan int
}

type HetroRoutines struct {
	ctrls     map[int]*_control
	ctrlsLock sync.Mutex
}

func NewHetroRoutines() *HetroRoutines {
	return &HetroRoutines{
		ctrls: make(map[int]*_control),
	}
}

func (me *HetroRoutines) New() (quit <-chan int, done chan<- int, idx int) {
	me.ctrlsLock.Lock()
	defer me.ctrlsLock.Unlock()

	// get an index
	for {
		idx = rand.Int()
		_, ok := me.ctrls[idx]
		if !ok {
			break
		}
	}

	me.ctrls[idx] = &_control{
		quit: make(chan int, 1),
		done: make(chan int, 1),
	}

	quit = me.ctrls[idx].quit
	done = me.ctrls[idx].done
	return
}

func (me *HetroRoutines) Stop(idx int) (err error) {
	me.ctrlsLock.Lock()
	defer me.ctrlsLock.Unlock()

	c, ok := me.ctrls[idx]
	if !ok {
		err = errors.New(fmt.Sprintf("Index not found: %v", idx))
		return
	}

	delete(me.ctrls, idx)
	c.quit <- 1
	close(c.quit)
	_, _ = <-c.done
	return
}

func (me *HetroRoutines) Close() {
	me.ctrlsLock.Lock()
	defer me.ctrlsLock.Unlock()

	// sending quit signal
	for _, v := range me.ctrls {
		v.quit <- 1
		close(v.quit)
	}

	// awaiting done signal
	for _, v := range me.ctrls {
		_, _ = <-v.done
	}

	me.ctrls = make(map[int]*_control)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
