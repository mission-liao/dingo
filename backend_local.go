package dingo

import (
	"errors"
	"fmt"
	"sync"
)

//
// configuration
//

type localBackend struct {
	cfg       *Config
	fromUser  chan *ReportEnvelope
	to        chan *ReportEnvelope // simulate the wire
	reporters *HetroRoutines
	reports   chan []byte
	stores    *Routines
	storeLock sync.Mutex
	isStored  bool
	toCheck   map[string]map[string]chan []byte // mapping (name, id) to report channel
	unSent    []*ReportEnvelope
}

/*
 A Backend implementation based on 'channel'. Users can provide a channel and
 share it between multiple Reporter(s) and Store(s) to connect them.
*/
func NewLocalBackend(cfg *Config, to chan *ReportEnvelope) (v *localBackend, err error) {
	v = &localBackend{
		cfg:       cfg,
		stores:    NewRoutines(),
		reporters: NewHetroRoutines(),
		fromUser:  to,
		to:        to,
		reports:   make(chan []byte, 10),
		toCheck:   make(map[string]map[string]chan []byte),
		unSent:    make([]*ReportEnvelope, 0, 10),
	}

	if v.to == nil {
		v.to = make(chan *ReportEnvelope, 10)
	}

	return
}

func (me *localBackend) _reporter_routine_(quit <-chan int, done chan<- int, events chan<- *Event, reports <-chan *ReportEnvelope) {
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

func (me *localBackend) _store_routine_(quit <-chan int, wait *sync.WaitGroup, events chan<- *Event) {
	defer wait.Done()

	out := func(enp *ReportEnvelope) {
		me.storeLock.Lock()
		defer me.storeLock.Unlock()

		found := false
		if ids, ok := me.toCheck[enp.ID.Name()]; ok {
			var ch chan []byte
			if ch, found = ids[enp.ID.ID()]; found {
				ch <- enp.Body
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
	for {
		select {
		case v, ok := <-me.to:
			if !ok {
				break clean
			}
			out(v)
		default:
			break clean
		}
	}
}

//
// Object interface
//

func (me *localBackend) Expect(types int) (err error) {
	if types&^(ObjT.REPORTER|ObjT.STORE) != 0 {
		err = errors.New(fmt.Sprintf("unsupported types: %v", types))
		return
	}

	if types&ObjT.STORE == ObjT.STORE {
		func() {
			me.storeLock.Lock()
			defer me.storeLock.Unlock()
			go me._store_routine_(me.stores.New(), me.stores.Wait(), me.stores.Events())
		}()
	}

	return
}

func (me *localBackend) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		me.reporters.Events(),
		me.stores.Events(),
	}, nil
}

func (me *localBackend) Close() (err error) {
	me.stores.Close()
	me.reporters.Close()

	close(me.reports)

	if me.fromUser == nil {
		close(me.to)
	}
	me.to = me.fromUser
	if me.to == nil {
		me.to = make(chan *ReportEnvelope, 10)
	}

	return
}

//
// Reporter
//

func (me *localBackend) ReporterHook(eventID int, payload interface{}) (err error) {
	return
}

func (me *localBackend) Report(reports <-chan *ReportEnvelope) (id int, err error) {
	quit, done, id := me.reporters.New(0)
	go me._reporter_routine_(quit, done, me.reporters.Events(), reports)

	return
}

//
// Store
//

func (me *localBackend) StoreHook(eventID int, payload interface{}) (err error) { return }
func (me *localBackend) Poll(meta Meta) (reports <-chan []byte, err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	if meta == nil {
		return
	}

	var (
		r    chan []byte
		id   string = meta.ID()
		name string = meta.Name()
	)

	found := false
	if ids, ok := me.toCheck[name]; ok {
		r, found = ids[id]
	}

	if !found {
		r = make(chan []byte, 10)
		ids, ok := me.toCheck[name]
		if !ok {
			ids = map[string]chan []byte{id: r}
			me.toCheck[name] = ids
		} else {
			ids[id] = r
		}
		reports = r
	}

	// reverse traversing when deleting in slice
	toSent := []*ReportEnvelope{}
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.ID.ID() == id && v.ID.Name() == name {
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

func (me *localBackend) Done(meta Meta) (err error) {
	var (
		id   string = meta.ID()
		name string = meta.Name()
	)

	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	// clearing toCheck list
	if ids, ok := me.toCheck[name]; ok {
		delete(ids, id)
	}

	// clearing unSent
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.ID.ID() == id && v.ID.Name() == name {
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}
	return
}
