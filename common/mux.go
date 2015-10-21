package common

//
// 'mux' is a n-to-1 multiplexer for a slice of 'receiving' channels.
// output is a "chan<- interface{}".
//
// the original use case in 'dingo' is muxing from chan<-task.TaskInfo from
// brokers and chan<-task.Report from backends.
//
// 'Mux' won't close those registered channels, but it would take care of
// its output channel, callers should check channel validity when receiving
// from 'Mux''s output channel:
//
//     m := &Mux{}
//     m.Init()
//       ...
//     out, err := m.Out()
//     v, ok := <-out
//

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"time"
)

type _newChannel struct {
	id int
	v  interface{}
}

// output of mux
type MuxOut struct {
	// the 'id' returned from Mux.Register
	Id    int
	Value interface{}
}

type Mux struct {
	ctrl *RtControl

	// check for new condition
	rw               sync.RWMutex
	cases            map[int]interface{}
	updated, touched time.Time

	// modifier
	lck      sync.Mutex
	_2delete []int
	_2add    []*_newChannel
	// output channel
	_out chan *MuxOut
}

//
func (m *Mux) Init() {
	m.cases = make(map[int]interface{})
	m.ctrl = NewRtCtrl()

	func() {
		m.lck.Lock()
		defer m.lck.Unlock()

		m.cases[0] = m.ctrl.Quit
		m.touched = time.Now()
	}()

	m._out = make(chan *MuxOut, 10)

	// mux routine
	go func() {
		var cond []reflect.SelectCase
		var keys []int
		for {
			// check for new arrival
			if m.updated.Before(m.touched) {
				func() {
					// writer lock
					m.rw.Lock()
					defer m.rw.Unlock()

					// locking
					m.lck.Lock()
					defer m.lck.Unlock()

					// remove / append based on _2add, _2delete
					for _, v := range m._2add {
						m.cases[v.id] = v.v
					}
					m._2add = make([]*_newChannel, 0, 10)

					for _, v := range m._2delete {
						delete(m.cases, v)
					}
					m._2delete = make([]int, 0, 10)

					// update timestamp
					m.updated = time.Now()
				}()

				func() {
					// reader lock
					m.rw.RLock()
					defer m.rw.RUnlock()

					// re-init sorted key slice
					keys = make([]int, 0, 10)
					for k := range m.cases {
						keys = append(keys, k)
					}
					sort.Ints(keys)

					// re-init a new condition slice
					cond = make([]reflect.SelectCase, 0, 10)
					for _, k := range keys {
						cond = append(cond, reflect.SelectCase{
							Dir:  reflect.SelectRecv,
							Chan: reflect.ValueOf(m.cases[k]),
						})
					}
				}()
			}

			// add a time.After channel
			// TODO: configuration for timeout
			cond = append(cond, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(time.After(1 * time.Second)),
			})

			// select...
			chosen, value, ok := reflect.Select(cond)
			cond = cond[:len(cond)-1] // pop the last timer event
			switch chosen {
			case 0:
				// quit channel is triggered,
				// need to clean-up && quit.
				m.ctrl.Done <- 1
				return
			case len(m.cases):
				// time-out event is triggered,
				// go for another round of for loop.
				continue
			default:
				if !ok {
					// that input channel is closed,
					// remove it
					func() {
						m.lck.Lock()
						defer m.lck.Unlock()

						m._2delete = append(m._2delete, keys[chosen])
						m.touched = time.Now()
					}()

					// its value is not trustable,
					// so go for another round of for loop.
					continue
				} else {
					// send to output channel
					if value.CanInterface() {
						m._out <- &MuxOut{
							Id:    keys[chosen],
							Value: value.Interface(),
						}
					}
				}
			}
		}
	}()
}

//
func (m *Mux) Close() {
	// routine control
	if m.ctrl != nil {
		m.ctrl.Close()
		m.ctrl = nil
	}

	if m._out != nil {
		close(m._out)
	}

	func() {
		m.rw.Lock()
		defer m.rw.Unlock()
		m.cases = nil
	}()
}

//
func (m *Mux) Register(ch interface{}) (id int, err error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	if m.cases == nil {
		err = errors.New("Mux: Not Initialized")
		return
	}

	m.lck.Lock()
	defer m.lck.Unlock()

	for {
		// generate a unique name
		id = rand.Int()
		_, ok := m.cases[id]
		if ok {
			// duplication found
			continue
		}

		found := false
		for _, v := range m._2add {
			if v.id == id {
				found = true
				break
			}
		}
		if found {
			continue
		}

		break
	}

	m._2add = append(m._2add, &_newChannel{id, ch})
	m.touched = time.Now()
	return
}

//
func (m *Mux) Unregister(id int) (ch interface{}, err error) {
	if id == 0 {
		err = errors.New("Mux: Unable to unregister quit channel")
		return
	}

	func() {
		m.rw.RLock()
		defer m.rw.RUnlock()

		_, ok := m.cases[id]
		if !ok {
			// look for that id in _2add
			found := false
			func() {
				m.lck.Lock()
				defer m.lck.Unlock()

				for k, v := range m._2add {
					if v.id == id {
						// remove that element
						m._2add = append(m._2add[:k], m._2add[k+1:]...)

						found = true
						break
					}
				}
			}()

			if !found {
				err = errors.New(fmt.Sprintf("Mux: '%q' not found", id))
				return
			}
		}
	}()

	func() {
		m.lck.Lock()
		defer m.lck.Unlock()

		m._2delete = append(m._2delete, id)
		m.touched = time.Now()
	}()

	return
}

//
func (m *Mux) Out() <-chan *MuxOut {
	return m._out
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
