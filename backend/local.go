package backend

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/task"
)

type _local struct {
	cBackend   *common.RtControl
	cSubscribe *common.RtControl
	to         chan []byte
	reports    chan task.Report
	reportLock sync.Mutex
	muxReport  *common.Mux
	toCheck    []string
	unSent     []task.Report
	subscriber map[string]chan<- task.Report
}

// factory
func newLocal() *_local {
	v := &_local{
		cBackend:   common.NewRtCtrl(),
		cSubscribe: common.NewRtCtrl(),
		to:         make(chan []byte, 10),
		reports:    make(chan task.Report, 10),
		muxReport:  &common.Mux{},
		toCheck:    make([]string, 0, 10),
		unSent:     make([]task.Report, 0, 10),
		subscriber: make(map[string]chan<- task.Report),
	}
	v.init()

	return v
}

func (me *_local) init() {
	me.muxReport.Init()

	// Reporter -> Store
	go func(reports <-chan *common.MuxOut, quit <-chan int, done chan<- int) {
		for {
			select {
			case _, _ = <-quit:
				done <- 1
				return
			case v, ok := <-reports:
				if !ok {
					// TODO:
				}

				rep, valid := v.Value.(task.Report)
				if !valid {
					// TODO:
				}

				body, err := json.Marshal(rep)
				if err != nil {
					// TODO:
				}

				// send to Store
				me.to <- body
			}
		}
	}(me.muxReport.Out(), me.cBackend.Quit, me.cBackend.Done)

	// Store -> Subscriber
	go func(quit <-chan int, done chan<- int) {
		for {
			select {
			case _, _ = <-quit:
				done <- 1
				return
			case v, ok := <-me.to:
				if !ok {
					// TODO:
				}

				rep, err := task.UnmarshalReport(v)
				if err != nil {
					// TODO:
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
	}(me.cSubscribe.Quit, me.cSubscribe.Done)
}

func (me *_local) Close() {
	me.cBackend.Close()
	me.cSubscribe.Close()
	me.muxReport.Close()

	close(me.reports)
	close(me.to)
}

//
// Reporter
//

func (me *_local) Report(report <-chan task.Report) (id string, err error) {
	v, err := me.muxReport.Register(report)
	if err != nil {
		return
	}

	id = fmt.Sprintf("%d", v)
	return
}

func (me *_local) Unbind(id string) (err error) {
	// convert string to int
	_id, err := strconv.Atoi(id)
	if err != nil {
		return
	}

	_, err = me.muxReport.Unregister(_id)
	return
}

//
// Store
//

func (me *_local) Subscribe() (reports <-chan task.Report, err error) {
	reports = me.reports
	return
}

func (me *_local) Poll(t task.Task) (err error) {
	me.reportLock.Lock()
	defer me.reportLock.Unlock()

	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.GetId() == t.GetId() {
			me.reports <- v
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}

	found := false
	for _, v := range me.toCheck {
		if v == t.GetId() {
			found = true
		}
	}

	if !found {
		me.toCheck = append(me.toCheck, t.GetId())
	}

	return
}

func (me *_local) Done(t task.Task) (err error) {
	// clearing toCheck list
	for k, v := range me.toCheck {
		if v == t.GetId() {
			me.toCheck = append(me.toCheck[:k], me.toCheck[k+1:]...)
			break
		}
	}

	// clearing unSent
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.GetId() == t.GetId() {
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}
	return
}
