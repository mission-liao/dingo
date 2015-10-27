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
	Bypass_ bool `json:"Bypass"`
}

func (me *_localConfig) Bypass(yes bool) *_localConfig {
	me.Bypass_ = yes
	return me
}

func defaultLocalConfig() *_localConfig {
	return &_localConfig{
		Bypass_: true,
	}
}

//
//
//

type _local struct {
	// broker routine
	brk    *common.Routines
	to     chan []byte
	noJSON chan meta.Task
	tasks  chan meta.Task
	errs   chan error
	bypass bool

	// monitor routine
	mnt        *common.Routines
	muxReceipt *common.Mux
	uhLock     sync.Mutex
	unhandled  map[string]meta.Task
	fLock      sync.RWMutex
	failures   []*Receipt
	rid        int
}

// factory
func newLocal(cfg *Config) (v *_local, err error) {
	v = &_local{
		brk:    common.NewRoutines(),
		to:     make(chan []byte, 10),
		noJSON: make(chan meta.Task, 10),
		tasks:  make(chan meta.Task, 10),
		errs:   make(chan error, 10),
		bypass: cfg.Local.Bypass_,

		mnt:        common.NewRoutines(),
		muxReceipt: common.NewMux(),
		unhandled:  make(map[string]meta.Task),
		failures:   make([]*Receipt, 0, 10),
		rid:        0,
	}

	v.init()
	return
}

func (me *_local) init() (err error) {
	// TODO: allow configuration
	_, err = me.muxReceipt.More(1)

	// broker routine
	quit, wait := me.brk.New()
	go me._broker_routine_(quit, wait)

	// start a new monitor routine
	quit, wait = me.mnt.New()
	go me._monitor_routine_(quit, wait)

	return
}

func (me *_local) _broker_routine_(quit <-chan int, wait *sync.WaitGroup) {
	defer wait.Done()

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
	for {
		select {
		case _, _ = <-quit:
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
}

func (me *_local) _monitor_routine_(quit <-chan int, wait *sync.WaitGroup) {
	defer wait.Done()
	for {
		select {
		case _, _ = <-quit:
			return
		case v, ok := <-me.muxReceipt.Out():
			if !ok {
				// mux is closed
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
}

//
// constructor / destructor
//

func (me *_local) Close() (err error) {
	me.brk.Close()
	me.mnt.Close()
	close(me.to)
	close(me.noJSON)
	close(me.tasks)
	close(me.errs)
	me.muxReceipt.Close()
	return
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

func (me *_local) AddListener(rcpt <-chan Receipt) (tasks <-chan meta.Task, errs <-chan error, err error) {
	me.rid, err = me.muxReceipt.Register(rcpt)
	tasks, errs = me.tasks, me.errs
	return
}

func (me *_local) Stop() (err error) {
	// reset tasks, errs
	me.tasks = make(chan meta.Task, 10)
	me.errs = make(chan error, 10)

	// reset receipts
	me.muxReceipt.Close()
	me.muxReceipt = common.NewMux()
	return
}
