package broker

import (
	"encoding/json"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

//
// configuration
//

type _localConfig struct {
	_bypass bool `json:"Bypass"`
}

func (me *_localConfig) Bypass(yes bool) *_localConfig {
	me._bypass = yes
	return me
}

func defaultLocalConfig() *_localConfig {
	return &_localConfig{
		_bypass: true,
	}
}

//
//
//

type _local struct {
	// broker routine
	brk    *common.RtControl
	to     chan []byte
	noJSON chan meta.Task
	tasks  chan meta.Task
	errs   chan error
	bypass bool

	// monitor routine
	monitor    *common.RtControl
	muxReceipt *common.Mux
	uhLock     sync.Mutex
	unhandled  map[string]meta.Task
	fLock      sync.RWMutex
	failures   []*Receipt
	rid        int
}

// factory
func newLocal(cfg *Config) (v *_local) {
	v = &_local{
		brk:    common.NewRtCtrl(),
		to:     make(chan []byte, 10),
		noJSON: make(chan meta.Task, 10),
		tasks:  make(chan meta.Task, 10),
		errs:   make(chan error, 10),
		bypass: cfg._local._bypass,

		monitor:    common.NewRtCtrl(),
		muxReceipt: &common.Mux{},
		unhandled:  make(map[string]meta.Task),
		failures:   make([]*Receipt, 0, 10),
		rid:        0,
	}

	v.init()
	return
}

func (me *_local) init() {
	me.muxReceipt.Init()

	// output function of broker routine
	out := func(t meta.Task) {
		me.tasks <- t

		func() {
			me.uhLock.Lock()
			defer me.uhLock.Unlock()

			me.unhandled[t.GetId()] = t
		}()

		return
	}

	// broker routine
	go func(quit <-chan int, done chan<- int) {
		for {
			select {
			case _, _ = <-quit:
				done <- 1
				return
			case v, ok := <-me.noJSON:
				if !ok {
					break
				}
				out(v)
			case v, ok := <-me.to:
				if !ok {
					// TODO: ??
					break
				}

				t_, err_ := meta.UnmarshalTask(v)
				if err_ != nil {
					me.errs <- err_
					break
				}
				out(t_)
			}
		}
	}(me.brk.Quit, me.brk.Done)

	// start a new monitor routine
	go func(quit <-chan int, done chan<- int) {
		for {
			select {
			case _, _ = <-quit:
				done <- 1
				return
			case v, ok := <-me.muxReceipt.Out():
				if !ok {
					// mux is closed
					done <- 1
					return
				}

				out, valid := v.Value.(Receipt)
				if !valid {
					// TODO: log it
					break
				}
				if out.Status != Status.OK {
					func() {
						me.fLock.Lock()
						defer me.fLock.Unlock()

						// TODO: providing interface to access
						// these errors

						// catch this error recepit
						me.failures = append(me.failures, &out)
					}()
				}

				func() {
					me.uhLock.Lock()
					defer me.uhLock.Unlock()

					// stop monitoring
					delete(me.unhandled, out.Id)
				}()
			}
		}
	}(me.monitor.Quit, me.monitor.Done)
}

//
// constructor / destructor
//

func (me *_local) Close() {
	me.brk.Close()
	me.monitor.Close()
	close(me.to)
	close(me.noJSON)
	close(me.tasks)
	close(me.errs)
	me.muxReceipt.Close()
}

//
// Producer
//

func (me *_local) Send(t meta.Task) (err error) {
	if me.bypass {
		me.noJSON <- t
		return
	}

	// marshal
	body, err := json.Marshal(t)
	if err != nil {
		return
	}

	me.to <- body
	return
}

//
// Consumer
//

func (me *_local) Consume(rcpt <-chan Receipt) (tasks <-chan meta.Task, errs <-chan error, err error) {
	me.rid, err = me.muxReceipt.Register(rcpt)
	tasks, errs = me.tasks, me.errs
	return
}

func (me *_local) Stop() (err error) {
	_, err = me.muxReceipt.Unregister(me.rid)
	me.rid = 0
	return
}
