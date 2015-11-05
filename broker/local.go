package broker

import (
	"encoding/json"
	"errors"
	"fmt"
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
	cfg *Config

	// broker routine
	brk    *common.Routines
	to     chan []byte
	noJSON chan meta.Task
	tasks  chan meta.Task

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
		cfg: cfg,

		brk:    common.NewRoutines(),
		to:     make(chan []byte, 10),
		noJSON: make(chan meta.Task, 10),
		tasks:  make(chan meta.Task, 10),

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
	quit := me.brk.New()
	go me._broker_routine_(quit, me.brk.Wait(), me.brk.Events())

	// start a new monitor routine
	quit = me.mnt.New()
	go me._monitor_routine_(quit, me.mnt.Wait(), me.mnt.Events())

	return
}

func (me *_local) _broker_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event) {
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
			goto cleanup
		case v, ok := <-me.noJSON:
			if !ok {
				goto cleanup
			}
			out(v)
		case v, ok := <-me.to:
			if !ok {
				goto cleanup
			}

			t_, err := meta.UnmarshalTask(v)
			if err != nil {
				events <- common.NewEventFromError(common.InstT.PRODUCER, err)
				break
			}
			out(t_)
		}
	}
cleanup:
}

func (me *_local) _monitor_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case v, ok := <-me.muxReceipt.Out():
			if !ok {
				// mux is closed
				goto cleanup
			}

			out, valid := v.Value.(Receipt)
			if !valid {
				events <- common.NewEventFromError(
					common.InstT.CONSUMER,
					errors.New(fmt.Sprintf("Invalid receipt received:%v", v)),
				)
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
cleanup:
}

//
// common.Object interface
//

func (me *_local) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.brk.Events(),
		me.mnt.Events(),
	}, nil
}

func (me *_local) Close() (err error) {
	me.brk.Close()
	me.mnt.Close()
	close(me.to)
	close(me.noJSON)
	close(me.tasks)
	me.muxReceipt.Close()
	return
}

//
// Producer
//

func (me *_local) Send(t meta.Task) (err error) {
	if me.cfg.Local.Bypass_ {
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

func (me *_local) AddListener(rcpt <-chan Receipt) (tasks <-chan meta.Task, err error) {
	me.rid, err = me.muxReceipt.Register(rcpt, 0)
	tasks = me.tasks
	return
}

func (me *_local) Stop() (err error) {
	// reset tasks, errs
	me.tasks = make(chan meta.Task, 10)

	// reset receipts
	me.muxReceipt.Close()
	me.muxReceipt = common.NewMux()
	return
}
