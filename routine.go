package dingo

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
	quits  []chan int
	qLock  sync.Mutex
	wg     sync.WaitGroup
	events chan *Event
}

func NewRoutines() *Routines {
	return &Routines{
		quits:  make([]chan int, 0, 10),
		events: make(chan *Event, 10),
	}
}

func (me *Routines) New() <-chan int {
	me.wg.Add(1)

	me.qLock.Lock()
	defer me.qLock.Unlock()

	me.quits = append(me.quits, make(chan int, 1))
	return me.quits[len(me.quits)-1]
}

func (me *Routines) Wait() *sync.WaitGroup {
	return &me.wg
}

func (me *Routines) Events() chan *Event {
	return me.events
}

// stop/release all allocated routines,
// this function should be safe from multiple calls.
func (me *Routines) Close() {
	me.qLock.Lock()
	defer me.qLock.Unlock()

	for _, v := range me.quits {
		v <- 1
		close(v)
	}
	me.quits = make([]chan int, 0, 10)
	me.wg.Wait()

	close(me.events)
	me.events = make(chan *Event, 10)
}

// Heterogeneous Routines
//
// similar to 'Routines', but can be closed
// one by one.

type _control struct {
	quit, done chan int
}

type HetroRoutines struct {
	ctrls     map[int]*_control
	ctrlsLock sync.Mutex
	events    chan *Event
}

func NewHetroRoutines() *HetroRoutines {
	return &HetroRoutines{
		ctrls:  make(map[int]*_control),
		events: make(chan *Event, 10),
	}
}

func (me *HetroRoutines) New(want int) (quit <-chan int, done chan<- int, idx int) {
	me.ctrlsLock.Lock()
	defer me.ctrlsLock.Unlock()

	// get an index
	idx = want
	for {
		_, ok := me.ctrls[idx]
		if !ok {
			break
		}
		idx = rand.Int()
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
	var c *_control
	err = func() (err error) {
		me.ctrlsLock.Lock()
		defer me.ctrlsLock.Unlock()

		var ok bool
		c, ok = me.ctrls[idx]
		if !ok {
			err = errors.New(fmt.Sprintf("Index not found: %v", idx))
			return
		}

		delete(me.ctrls, idx)
		return
	}()

	if c != nil {
		c.quit <- 1
		close(c.quit)
		_, _ = <-c.done
	}
	return
}

func (me *HetroRoutines) Events() chan *Event {
	return me.events
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
