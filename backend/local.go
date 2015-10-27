package backend

// TODO: bypass mode in local backend

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

type _local struct {
	cBackend   *common.Routines
	cSubscribe *common.Routines
	to         chan []byte
	reports    chan meta.Report
	reportLock sync.Mutex
	muxReport  *common.Mux
	rid        int
	toCheck    []string
	unSent     []meta.Report
	subscriber map[string]chan<- meta.Report
}

// factory
func newLocal(cfg *Config) (v *_local, err error) {
	v = &_local{
		cBackend:   common.NewRoutines(),
		cSubscribe: common.NewRoutines(),
		to:         make(chan []byte, 10),
		reports:    make(chan meta.Report, 10),
		muxReport:  common.NewMux(),
		toCheck:    make([]string, 0, 10),
		unSent:     make([]meta.Report, 0, 10),
		subscriber: make(map[string]chan<- meta.Report),
	}
	err = v.init()
	return
}

func (me *_local) _reporter_routine_(reports <-chan *common.MuxOut, quit <-chan int, wait *sync.WaitGroup) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			return
		case v, ok := <-reports:
			if !ok {
				// TODO:
			}

			rep, valid := v.Value.(meta.Report)
			if !valid {
				// TODO:
			}

			body, err := json.Marshal(rep)
			if err != nil {
				// TODO: an error channel to reports errors
			}

			// send to Store
			me.to <- body
		}
	}
}

func (me *_local) _store_routine_(quit <-chan int, wait *sync.WaitGroup) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			return
		case v, ok := <-me.to:
			if !ok {
				// TODO:
			}

			rep, err := meta.UnmarshalReport(v)
			if err != nil {
				// TODO:
				break
			}

			if rep == nil {
				break
			}

			func() {
				me.reportLock.Lock()
				me.reportLock.Unlock()

				found := false
				for _, v := range me.toCheck {
					if v == rep.GetId() {
						found = true
						me.reports <- rep
						break
					}
				}

				if !found {
					me.unSent = append(me.unSent, rep)
				}
			}()
		}
	}
}

func (me *_local) init() (err error) {
	// TODO: allow configuration
	_, err = me.muxReport.More(1)

	// Reporter -> Store
	quit, wait := me.cBackend.New()
	go me._reporter_routine_(me.muxReport.Out(), quit, wait)

	// Store -> Subscriber
	quit, wait = me.cSubscribe.New()
	go me._store_routine_(quit, wait)

	return
}

func (me *_local) Close() (err error) {
	me.cBackend.Close()
	me.cSubscribe.Close()
	me.muxReport.Close()

	close(me.reports)
	close(me.to)

	return
}

//
// Reporter
//

func (me *_local) Report(reports <-chan meta.Report) (err error) {
	me.rid, err = me.muxReport.Register(reports)
	return
}

func (me *_local) Unbind() (err error) {
	// convert string to int
	_, err = me.muxReport.Unregister(me.rid)
	return
}

//
// Store
//

func (me *_local) Subscribe() (reports <-chan meta.Report, err error) {
	reports = me.reports
	return
}

func (me *_local) Poll(id meta.ID) (err error) {
	me.reportLock.Lock()
	defer me.reportLock.Unlock()

	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.GetId() == id.GetId() {
			me.reports <- v
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}

	found := false
	for _, v := range me.toCheck {
		if v == id.GetId() {
			found = true
		}
	}

	if !found {
		me.toCheck = append(me.toCheck, id.GetId())
	}

	return
}

func (me *_local) Done(id meta.ID) (err error) {
	// clearing toCheck list
	for k, v := range me.toCheck {
		if v == id.GetId() {
			me.toCheck = append(me.toCheck[:k], me.toCheck[k+1:]...)
			break
		}
	}

	// clearing unSent
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.GetId() == id.GetId() {
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}
	return
}
