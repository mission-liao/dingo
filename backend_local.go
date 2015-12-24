package dingo

import (
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

/*NewLocalBackend allocates a Backend implementation based on 'channel'. Users can provide a channel and
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

func (bkd *localBackend) reporterRoutine(quit <-chan int, done chan<- int, events chan<- *Event, reports <-chan *ReportEnvelope) {
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
			bkd.to <- v
		}
	}
clean:
}

func (bkd *localBackend) storeRoutine(quit <-chan int, wait *sync.WaitGroup, events chan<- *Event) {
	defer wait.Done()

	out := func(enp *ReportEnvelope) {
		bkd.storeLock.Lock()
		defer bkd.storeLock.Unlock()

		found := false
		if ids, ok := bkd.toCheck[enp.ID.Name()]; ok {
			var ch chan []byte
			if ch, found = ids[enp.ID.ID()]; found {
				ch <- enp.Body
			}
		}

		if !found {
			bkd.unSent = append(bkd.unSent, enp)
		}
	}

	for {
		select {
		case _, _ = <-quit:
			goto clean
		case v, ok := <-bkd.to:
			if !ok {
				goto clean
			}
			out(v)
		}
	}
clean:
	for {
		select {
		case v, ok := <-bkd.to:
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

func (bkd *localBackend) Expect(types int) (err error) {
	if types&^(ObjT.Reporter|ObjT.Store) != 0 {
		err = fmt.Errorf("unsupported types: %v", types)
		return
	}

	if types&ObjT.Store == ObjT.Store {
		func() {
			bkd.storeLock.Lock()
			defer bkd.storeLock.Unlock()
			go bkd.storeRoutine(bkd.stores.New(), bkd.stores.Wait(), bkd.stores.Events())
		}()
	}

	return
}

func (bkd *localBackend) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		bkd.reporters.Events(),
		bkd.stores.Events(),
	}, nil
}

func (bkd *localBackend) Close() (err error) {
	bkd.stores.Close()
	bkd.reporters.Close()

	close(bkd.reports)

	if bkd.fromUser == nil {
		close(bkd.to)
	}
	bkd.to = bkd.fromUser
	if bkd.to == nil {
		bkd.to = make(chan *ReportEnvelope, 10)
	}

	return
}

//
// Reporter
//

func (bkd *localBackend) ReporterHook(eventID int, payload interface{}) (err error) {
	return
}

func (bkd *localBackend) Report(reports <-chan *ReportEnvelope) (id int, err error) {
	quit, done, id := bkd.reporters.New(0)
	go bkd.reporterRoutine(quit, done, bkd.reporters.Events(), reports)

	return
}

//
// Store
//

func (bkd *localBackend) StoreHook(eventID int, payload interface{}) (err error) { return }
func (bkd *localBackend) Poll(meta Meta) (reports <-chan []byte, err error) {
	bkd.storeLock.Lock()
	defer bkd.storeLock.Unlock()

	if meta == nil {
		return
	}

	var (
		r    chan []byte
		id   = meta.ID()
		name = meta.Name()
	)

	found := false
	if ids, ok := bkd.toCheck[name]; ok {
		r, found = ids[id]
	}

	if !found {
		r = make(chan []byte, 10)
		ids, ok := bkd.toCheck[name]
		if !ok {
			ids = map[string]chan []byte{id: r}
			bkd.toCheck[name] = ids
		} else {
			ids[id] = r
		}
		reports = r
	}

	// reverse traversing when deleting in slice
	toSent := []*ReportEnvelope{}
	for i := len(bkd.unSent) - 1; i >= 0; i-- {
		v := bkd.unSent[i]
		if v.ID.ID() == id && v.ID.Name() == name {
			// prepend
			toSent = append([]*ReportEnvelope{v}, toSent...)
			// delete this element
			bkd.unSent = append(bkd.unSent[:i], bkd.unSent[i+1:]...)
		}
	}

	for _, v := range toSent {
		r <- v.Body
	}

	return
}

func (bkd *localBackend) Done(meta Meta) (err error) {
	var (
		id   = meta.ID()
		name = meta.Name()
	)

	bkd.storeLock.Lock()
	defer bkd.storeLock.Unlock()

	// clearing toCheck list
	if ids, ok := bkd.toCheck[name]; ok {
		delete(ids, id)
	}

	// clearing unSent
	for i := len(bkd.unSent) - 1; i >= 0; i-- {
		v := bkd.unSent[i]
		if v.ID.ID() == id && v.ID.Name() == name {
			// delete this elebkdnt
			bkd.unSent = append(bkd.unSent[:i], bkd.unSent[i+1:]...)
		}
	}
	return
}
