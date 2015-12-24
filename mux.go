package dingo

//
// 'mux' is a n-to-1 multiplexer for a slice of 'receiving' channels.
// Users can add handler function to handle those input values.
//
// the original use case in 'dingo' is muxing from chan<-task.TaskInfo from
// brokers and chan<-task.Report from backends.
//
// 'mux' won't close those registered channels, but it would take care of
// its output channel, callers should check channel validity when receiving
// from 'mux''s output channel:
//
//     m := &mux{}
//     m.Init()
//       ...
//     m.Handle(func(v interface{}, idx int) {
//         // output it to another channel
//         out <- v.(string)
//     })
//

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type _newChannel struct {
	id int
	v  interface{}
}

type mux struct {
	rs      *Routines
	changed []chan time.Time
	rsLock  sync.Mutex

	// check for new condition
	cases        atomic.Value
	casesLock    sync.Mutex
	handlersLock sync.Mutex
	handlers     atomic.Value
}

func newMux() (m *mux) {
	m = &mux{
		rs:      NewRoutines(),
		changed: make([]chan time.Time, 0, 10),
	}

	m.cases.Store(make(map[int]interface{}))
	m.handlers.Store(make([]func(interface{}, int), 0, 10))
	return
}

//
func (mx *mux) More(count int) (remain int, err error) {
	remain = count
	for ; remain > 0; remain-- {
		c := make(chan time.Time, 10)
		mx.changed = append(mx.changed, c)
		go mx.muxRoutine(mx.rs.New(), mx.rs.Wait(), c)
	}
	return
}

//
func (mx *mux) Close() {
	func() {
		mx.rsLock.Lock()
		defer mx.rsLock.Unlock()
		mx.rs.Close()

		for _, v := range mx.changed {
			close(v)
		}
		mx.changed = make([]chan time.Time, 0, 10)
	}()

	mx.casesLock.Lock()
	defer mx.casesLock.Unlock()
	mx.cases.Store(make(map[int]interface{}))

	mx.handlersLock.Lock()
	defer mx.handlersLock.Unlock()
	mx.handlers.Store(make([]func(interface{}, int), 0, 10))
}

//
func (mx *mux) Register(ch interface{}, expectedID int) (id int, err error) {
	func() {
		mx.casesLock.Lock()
		defer mx.casesLock.Unlock()

		m := mx.cases.Load().(map[int]interface{})
		id = expectedID
		for {
			if _, ok := m[id]; !ok {
				break
			}

			id = rand.Int()
		}

		nm := make(map[int]interface{})
		for k := range m {
			nm[k] = m[k]
		}
		nm[id] = ch
		mx.cases.Store(nm)
	}()

	mx.rsLock.Lock()
	defer mx.rsLock.Unlock()

	touched := time.Now()
	for _, v := range mx.changed {
		v <- touched
	}
	return
}

//
func (mx *mux) Unregister(id int) (ch interface{}, err error) {
	func() {
		mx.casesLock.Lock()
		defer mx.casesLock.Unlock()

		var ok bool
		m := mx.cases.Load().(map[int]interface{})
		if ch, ok = m[id]; !ok {
			err = fmt.Errorf("Id not found:%v", id)
			return
		}
		delete(m, id)

		nm := make(map[int]interface{})
		for k := range m {
			nm[k] = m[k]
		}
		mx.cases.Store(nm)
	}()

	mx.rsLock.Lock()
	defer mx.rsLock.Unlock()

	touched := time.Now()
	for _, v := range mx.changed {
		v <- touched
	}
	return
}

func (mx *mux) Handle(handler func(interface{}, int)) {
	func() {
		mx.handlersLock.Lock()
		defer mx.handlersLock.Unlock()

		m := mx.handlers.Load().([]func(interface{}, int))
		nm := make([]func(interface{}, int), 0, len(m)+1)
		copy(nm, m)
		nm = append(nm, handler)
		mx.handlers.Store(nm)
	}()

	mx.rsLock.Lock()
	defer mx.rsLock.Unlock()

	touched := time.Now()
	for _, v := range mx.changed {
		v <- touched
	}
}

func (mx *mux) muxRoutine(quit <-chan int, wait *sync.WaitGroup, changed <-chan time.Time) {
	defer wait.Done()
	var (
		cond       []reflect.SelectCase
		handlers   []func(interface{}, int)
		keys       []int
		lenOfcases int
	)

	del := func(chosen int) {
		cond = append(cond[:chosen], cond[chosen+1:]...)
		keys = append(keys[:chosen], keys[chosen+1:]...)
		lenOfcases--
	}

	update := func() {
		m := mx.cases.Load().(map[int]interface{})

		keys = make([]int, 0, 10)
		for k := range m {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		cond = make([]reflect.SelectCase, 0, 10)
		for _, k := range keys {
			cond = append(cond, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(m[k]),
			})
		}

		// add quit channel
		cond = append(cond, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(quit),
		})
		// add changed channel
		cond = append(cond, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(changed),
		})

		lenOfcases = len(m)

		// update handlers
		handlers = mx.handlers.Load().([]func(interface{}, int))
	}

	update()
	for {
		chosen, value, ok := reflect.Select(cond)
		if !ok {
			// control channel is closed (quit, changed)
			if chosen >= lenOfcases {
				goto cleanup
			}

			// remove that channel
			del(chosen)

			// its value is not trustable,
			// so go for another round of for loop.
			continue
		}

		switch chosen {

		// quit channel is triggered.
		case lenOfcases:
			goto cleanup

		// changed channel is triggered
		case lenOfcases + 1:
			// clear remaining changed event
			cleared := false
			for {
				select {
				case <-changed:
				default:
					cleared = true
				}
				if cleared {
					break
				}
			}
			update()

		// other registered channels
		default:
			// send to handlers
			for _, v := range handlers {
				v(value.Interface(), keys[chosen])
			}
		}
	}
cleanup:
	// update for the last time
	update()
	cond = cond[:len(cond)-2] // pop quit, changed channel
	cond = append(cond, reflect.SelectCase{
		Dir: reflect.SelectDefault,
	}) // append a default case

	// consuming things remaining in channels,
	// until cleared.
	for {
		chosen, value, ok := reflect.Select(cond)
		// note: when default case is triggered,
		// 'ok' is always false, which is meaningless.
		if !ok && chosen < len(cond)-1 {
			// remove that channel
			del(chosen)
			continue
		}

		switch chosen {
		// default is triggered
		case len(cond) - 1:
			return
		default:
			for _, v := range handlers {
				v(value.Interface(), keys[chosen])
			}
		}
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
