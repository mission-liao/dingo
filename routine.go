package dingo

//
// 'routines' helps to manage go-routines
// depending on the same input/output channels
//

import (
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

func (rs *Routines) New() <-chan int {
	rs.wg.Add(1)

	rs.qLock.Lock()
	defer rs.qLock.Unlock()

	rs.quits = append(rs.quits, make(chan int, 1))
	return rs.quits[len(rs.quits)-1]
}

func (rs *Routines) Wait() *sync.WaitGroup {
	return &rs.wg
}

func (rs *Routines) Events() chan *Event {
	return rs.events
}

/*Close is used to stop/release all allocated routines,
this function should be safe from multiple calls.
*/
func (rs *Routines) Close() {
	rs.qLock.Lock()
	defer rs.qLock.Unlock()

	for _, v := range rs.quits {
		v <- 1
		close(v)
	}
	rs.quits = make([]chan int, 0, 10)
	rs.wg.Wait()

	close(rs.events)
	rs.events = make(chan *Event, 10)
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

func (rs *HetroRoutines) New(want int) (quit <-chan int, done chan<- int, idx int) {
	rs.ctrlsLock.Lock()
	defer rs.ctrlsLock.Unlock()

	// get an index
	idx = want
	for {
		_, ok := rs.ctrls[idx]
		if !ok {
			break
		}
		idx = rand.Int()
	}

	rs.ctrls[idx] = &_control{
		quit: make(chan int, 1),
		done: make(chan int, 1),
	}

	quit = rs.ctrls[idx].quit
	done = rs.ctrls[idx].done
	return
}

func (rs *HetroRoutines) Stop(idx int) (err error) {
	var c *_control
	err = func() (err error) {
		rs.ctrlsLock.Lock()
		defer rs.ctrlsLock.Unlock()

		var ok bool
		c, ok = rs.ctrls[idx]
		if !ok {
			err = fmt.Errorf("Index not found: %v", idx)
			return
		}

		delete(rs.ctrls, idx)
		return
	}()

	if c != nil {
		c.quit <- 1
		close(c.quit)
		_, _ = <-c.done
	}
	return
}

func (rs *HetroRoutines) Events() chan *Event {
	return rs.events
}

func (rs *HetroRoutines) Close() {
	rs.ctrlsLock.Lock()
	defer rs.ctrlsLock.Unlock()

	// sending quit signal
	for _, v := range rs.ctrls {
		v.quit <- 1
		close(v.quit)
	}

	// awaiting done signal
	for _, v := range rs.ctrls {
		_, _ = <-v.done
	}

	rs.ctrls = make(map[int]*_control)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
