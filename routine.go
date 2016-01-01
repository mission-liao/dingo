package dingo

//
// 'routines' helps to manage go-routines
// depending on the same input/output channels
//

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
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
		if c, ok = rs.ctrls[idx]; !ok {
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

// Chained Routines
//
// routines act like a linked list, each node have prev/next channel to connect with each other

type nodeHandler interface {
	HandleInput(v interface{})
	HandleLink(v interface{}) bool
	Done()
}

type chainRoutines struct {
	head, tail         chan interface{}
	outterEvents       chan<- *Event
	remain             func(interface{})
	lock               sync.Mutex
	headWait, nodeWait sync.WaitGroup
}

func newChainRoutines(remain func(interface{}), events chan<- *Event) (v *chainRoutines) {
	v = &chainRoutines{
		head:         make(chan interface{}, 10), // TODO: config
		remain:       remain,
		outterEvents: events,
	}
	v.tail = v.head
	v.headWait.Add(1)
	go v.headRoutine(&v.headWait)
	return
}

func (rt *chainRoutines) Send(v interface{}) {
	rt.head <- v
}

func (rt *chainRoutines) Add(input interface{}, handler nodeHandler) error {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	if rt.head == nil || rt.tail == nil {
		return errors.New("chain-routines closed")
	}

	// prepare prev/next link for new node
	var (
		prev <-chan interface{}
		next chan<- interface{}
	)
	if rt.head == rt.tail {
		rt.head = make(chan interface{}, 10) // TODO: config
		prev, next = rt.head, rt.tail
	} else {
		next = rt.head
		rt.head = make(chan interface{}, 10)
		prev = rt.head
	}
	rt.nodeWait.Add(1)
	go rt.nodeRoutine(&rt.nodeWait, input, prev, next, handler)
	return nil
}

func (rt *chainRoutines) Close() {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	if rt.head == nil || rt.tail == nil {
		return
	}

	tmp := rt.head
	rt.head = make(chan interface{}, 100) // TODO: config
	// trigger a serious of close signal
	close(tmp)

	// wait until node routines done their clean up
	rt.nodeWait.Wait()

	// wait until head routine done
	rt.headWait.Wait()

	close(rt.head) // close the temporary channel
	rt.head = nil
	rt.tail = nil

	return
}

func (rt *chainRoutines) headRoutine(wait *sync.WaitGroup) {
	var (
		k    int
		v    interface{}
		ok   bool
		rest = make([]interface{}, 0, 100)
	)
	defer wait.Done()

	for {
		select {
		case <-time.After(1 * time.Millisecond):
			if rt.tail == rt.head {
				break
			}

			k = 0
		sent:
			for _, v = range rest {
				select {
				case rt.head <- v:
					k++
				default:
					// channel buffer is full
					break sent
				}
			}
			rest = rest[k:]
		case v, ok = <-rt.tail:
			if !ok {
				goto cleanTail
			}
			rest = append(rest, v)
		}
	}
cleanTail:
	for {
		// consume from head to 'rest'
		select {
		case v, ok = <-rt.tail:
			if !ok {
				break cleanTail
			}
			rest = append(rest, v)
		default:
			break cleanTail
		}
	}
cleanHead:
	for {
		// consume from head to 'rest'
		select {
		case v, ok = <-rt.head:
			if !ok {
				break cleanHead
			}
			rest = append(rest, v)
		default:
			break cleanHead
		}
	}

	// dump everythin remaining in 'rest'
	for _, v = range rest {
		rt.outterEvents <- NewEventFromError(ObjT.ChainRoutine, fmt.Errorf("remaining link event:%v", v))
		rt.remain(v)
	}
}

func (rt *chainRoutines) nodeRoutine(
	wait *sync.WaitGroup,
	input interface{},
	prev <-chan interface{},
	next chan<- interface{},
	handler nodeHandler,
) {
	defer func() {
		handler.Done()
		wait.Done()
	}()

	var (
		value  reflect.Value
		chosen int
		ok     bool
		v      interface{}
		conds  []reflect.SelectCase
	)

	// compose select-cases
	conds = []reflect.SelectCase{
		reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(prev),
		}, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(input),
		},
	}

	for {
		chosen, value, ok = reflect.Select(conds)
		switch chosen {
		case 0:
			// prev
			if !ok {
				goto clean
			}
			v = value.Interface()
			if !handler.HandleLink(v) {
				next <- v
			}
		case 1:
			// input
			if !ok {
				goto cleanLink
			}
			handler.HandleInput(value.Interface())
		}
	}
cleanLink:
	// stay alive to forward link packets, until link channel closed
	for {
		select {
		case v, ok = <-prev:
			if !ok {
				goto clean
			}
			if !handler.HandleLink(v) {
				next <- v
			}
		}
	}
clean:
	// trigger closing signal for next node
	close(next)

	// keep consuming remaining inputs
	conds = []reflect.SelectCase{
		reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(input),
		},
		reflect.SelectCase{
			Dir: reflect.SelectDefault,
		},
	}
finished:
	for {
		chosen, value, ok = reflect.Select(conds)
		switch chosen {
		case 0:
			if !ok {
				break finished
			}
			handler.HandleInput(value.Interface())
		case 1:
			break finished
		}
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
