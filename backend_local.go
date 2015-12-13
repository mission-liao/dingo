package dingo

import (
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
)

//
// configuration
//

type localBackend struct {
	cfg       *Config
	to        chan *ReportEnvelope // simulate the wire
	reporters *common.HetroRoutines
	reports   chan []byte
	stores    *common.Routines
	storeLock sync.Mutex
	toCheck   map[string]chan []byte
	unSent    []*ReportEnvelope
}

// factory
func NewLocalBackend(cfg *Config) (v *localBackend, err error) {
	v = &localBackend{
		cfg:       cfg,
		stores:    common.NewRoutines(),
		reporters: common.NewHetroRoutines(),
		to:        make(chan *ReportEnvelope, 10),
		reports:   make(chan []byte, 10),
		toCheck:   make(map[string]chan []byte),
		unSent:    make([]*ReportEnvelope, 0, 10),
	}

	go v._store_routine_(v.stores.New(), v.stores.Wait(), v.stores.Events())
	return
}

func (me *localBackend) _reporter_routine_(quit <-chan int, done chan<- int, events chan<- *common.Event, reports <-chan *ReportEnvelope) {
	defer func() {
		done <- 1
	}()

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case v, ok := <-reports:
			if !ok {
				goto clean
			}

			// send to Store
			me.to <- v
		}
	}
clean:
}

func (me *localBackend) _store_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *common.Event) {
	defer wait.Done()

	out := func(enp *ReportEnvelope) {
		me.storeLock.Lock()
		defer me.storeLock.Unlock()

		found := false
		for k, v := range me.toCheck {
			if k == enp.ID.ID() {
				found = true
				v <- enp.Body
				break
			}
		}

		if !found {
			me.unSent = append(me.unSent, enp)
		}
	}

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case v, ok := <-me.to:
			if !ok {
				goto clean
			}

			out(v)
		}
	}
clean:
}

//
// common.Object interface
//

func (me *localBackend) Events() ([]<-chan *common.Event, error) {
	return []<-chan *common.Event{
		me.reporters.Events(),
		me.stores.Events(),
	}, nil
}

func (me *localBackend) Close() (err error) {
	me.stores.Close()
	me.reporters.Close()

	close(me.reports)
	close(me.to)

	return
}

//
// Reporter
//

func (me *localBackend) Report(reports <-chan *ReportEnvelope) (id int, err error) {
	quit, done, id := me.reporters.New(0)
	go me._reporter_routine_(quit, done, me.reporters.Events(), reports)

	return
}

//
// Store
//

func (me *localBackend) Poll(id transport.Meta) (reports <-chan []byte, err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	var r chan []byte

	found := false
	for k, v := range me.toCheck {
		if k == id.ID() {
			found, r = true, v
		}
	}

	if !found {
		r = make(chan []byte, 10)
		me.toCheck[id.ID()], reports = r, r
	}

	// reverse traversing when deleting in slice
	toSent := []*ReportEnvelope{}
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.ID.ID() == id.ID() {
			// prepend
			toSent = append([]*ReportEnvelope{v}, toSent...)
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}

	for _, v := range toSent {
		r <- v.Body
	}

	return
}

func (me *localBackend) Done(id transport.Meta) (err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	// clearing toCheck list
	delete(me.toCheck, id.ID())

	// clearing unSent
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.ID.ID() == id.ID() {
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}
	return
}
