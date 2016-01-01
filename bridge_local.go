package dingo

import (
	"errors"
	"fmt"
	"sync"
)

type localStorePoller struct {
	task    *Task
	reports chan<- *Report
}

type localBridge struct {
	objLock   sync.RWMutex
	supported int
	broker    chan *Task
	listeners *Routines
	reporters map[string]*chainRoutines
	events    chan *Event
}

func newLocalBridge(args ...interface{}) (b bridge) {
	v := &localBridge{
		events:    make(chan *Event, 10),
		listeners: NewRoutines(),
		reporters: make(map[string]*chainRoutines),
		broker:    make(chan *Task, 100), // TODO: config
		supported: ObjT.Reporter | ObjT.Store | ObjT.Producer | ObjT.Consumer,
	}
	b = v

	return
}

func (bdg *localBridge) Events() ([]<-chan *Event, error) {
	return []<-chan *Event{
		bdg.events,
	}, nil
}

func (bdg *localBridge) Close() (err error) {
	bdg.objLock.Lock()
	defer bdg.objLock.Unlock()

	bdg.listeners.Close()
	for _, v := range bdg.reporters {
		v.Close()
	}

	close(bdg.broker)
	bdg.broker = make(chan *Task, 100) // TODO: config
	close(bdg.events)
	bdg.events = make(chan *Event, 100) // TODO: config
	return
}

func (bdg *localBridge) SendTask(t *Task) (err error) {
	bdg.objLock.RLock()
	defer bdg.objLock.RUnlock()

	bdg.broker <- t
	return
}

func (bdg *localBridge) AddNamedListener(name string, rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	err = errors.New("named consumer is not supported by local-bridge")
	return
}

func (bdg *localBridge) AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error) {
	bdg.objLock.RLock()
	defer bdg.objLock.RUnlock()

	tasks2 := make(chan *Task, 10)
	tasks = tasks2

	go func(
		quit <-chan int,
		wait *sync.WaitGroup,
		events chan<- *Event,
		input <-chan *Task,
		output chan<- *Task,
		receipts <-chan *TaskReceipt,
	) {
		defer wait.Done()
		var (
			reply *TaskReceipt
			ok    bool
		)
		out := func(t *Task) (done bool) {
			output <- t
			if reply, ok = <-receipts; !ok {
				done = true
				return
			}
			if reply.ID != t.ID() {
				events <- NewEventFromError(
					ObjT.Consumer,
					fmt.Errorf("expect receipt from %v, but %v", t, reply),
				)
				return
			}
			if reply.Status == ReceiptStatus.WorkerNotFound {
				events <- NewEventFromError(
					ObjT.Consumer,
					fmt.Errorf("workers not found: %v", t),
				)
				return
			}

			return
		}
		for {
			select {
			case _, _ = <-quit:
				goto clean
			case t, ok := <-input:
				if !ok {
					goto clean
				}
				if out(t) {
					goto clean
				}
			}
		}
	clean:
		for {
			select {
			case t, ok := <-input:
				if !ok {
					break clean
				}
				if out(t) {
					break clean
				}
			default:
				break clean
			}
		}
		close(output)
	}(bdg.listeners.New(), bdg.listeners.Wait(), bdg.events, bdg.broker, tasks2, rcpt)

	return
}

func (bdg *localBridge) StopAllListeners() (err error) {
	bdg.objLock.Lock()
	defer bdg.objLock.Unlock()

	bdg.listeners.Close()
	return
}

type localReporterNode struct {
	watched map[string]*localStorePoller // map id to poller
	unSent  map[string][]*Report         // map id to unsent reports
	name    string
	events  chan<- *Event
}

func (t *localReporterNode) HandleInput(v interface{}) {
	r := v.(*Report)
	id := r.ID()

	if p, ok := t.watched[id]; ok {
		p.reports <- r
		if r.Done() {
			delete(t.watched, id)
			close(p.reports)
		}
	} else {
		if rs, ok := t.unSent[id]; ok {
			t.unSent[id] = append(rs, r)
		} else {
			t.unSent[id] = []*Report{r}
		}
	}
}
func (t *localReporterNode) HandleLink(v interface{}) (ok bool) {
	var (
		p    = v.(*localStorePoller)
		unst []*Report
	)
	id := p.task.ID()

	if unst, ok = t.unSent[id]; ok {
		t.watched[id] = p
		for _, r := range unst {
			p.reports <- r
			if r.Done() {
				delete(t.watched, id)
				close(p.reports)
			}
		}
	}
	return
}

func (t *localReporterNode) Done() {
cleanWatched:
	for k, v := range t.watched {
		select {
		case t.events <- NewEventFromError(
			ObjT.Store,
			fmt.Errorf("unclosed reports channel: %v:%v", t.name, k),
		):
		default:
			break cleanWatched
		}

		// send a 'Shutdown' report
		r, err := v.task.composeReport(Status.Fail, nil, NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
		if err != nil {
			t.events <- NewEventFromError(ObjT.Store, err)
		} else {
			v.reports <- r
		}

		// remember to send t close signal
		close(v.reports)
	}
cleanUnSent:
	for _, v := range t.unSent {
		for _, r := range v {
			select {
			case t.events <- NewEventFromError(
				ObjT.Store,
				fmt.Errorf("unsent report: %v:%v", t.name, r),
			):
			default:
				break cleanUnSent
			}
		}
	}
}

func (bdg *localBridge) Report(name string, reports <-chan *Report) (err error) {
	bdg.objLock.Lock()
	defer bdg.objLock.Unlock()

	var (
		chain *chainRoutines
		ok    bool
	)

	if chain, ok = bdg.reporters[name]; !ok {
		chain = newChainRoutines(func(v interface{}) {
			poller := v.(*localStorePoller)
			// send a 'Shutdown' report
			r, err := poller.task.composeReport(Status.Fail, nil, NewErr(ErrCode.Shutdown, errors.New("dingo is shutdown")))
			if err != nil {
				bdg.events <- NewEventFromError(ObjT.Store, err)
			} else {
				poller.reports <- r
			}
		}, bdg.events)
		bdg.reporters[name] = chain
	}

	err = chain.Add(reports, &localReporterNode{
		watched: make(map[string]*localStorePoller),
		unSent:  make(map[string][]*Report),
		name:    name,
		events:  bdg.events,
	})

	return
}

func (bdg *localBridge) Poll(t *Task) (reports <-chan *Report, err error) {
	reports2 := make(chan *Report, Status.Count)
	reports = reports2

	bdg.objLock.Lock()
	defer bdg.objLock.Unlock()

	if chain, ok := bdg.reporters[t.Name()]; ok {
		chain.Send(&localStorePoller{
			task:    t,
			reports: reports2,
		})
	} else {
		err = fmt.Errorf("reporterNode found in local mode: %v", t.Name())
	}

	return
}

func (bdg *localBridge) AttachReporter(r Reporter) (err error) {
	return
}

func (bdg *localBridge) AttachStore(s Store) (err error) {
	return
}

func (bdg *localBridge) AttachProducer(p Producer) (err error) {
	return
}

func (bdg *localBridge) AttachConsumer(c Consumer, nc NamedConsumer) (err error) {
	return
}

func (bdg *localBridge) Exists(it int) bool {
	// make sure only one component is selected
	switch it {
	case ObjT.Producer:
		return bdg.supported&it == it
	case ObjT.Consumer:
		return bdg.supported&it == it
	case ObjT.Reporter:
		return bdg.supported&it == it
	case ObjT.Store:
		return bdg.supported&it == it
	}

	return false
}

func (bdg *localBridge) ReporterHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (bdg *localBridge) StoreHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (bdg *localBridge) ProducerHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}

func (bdg *localBridge) ConsumerHook(eventID int, payload interface{}) (err error) {
	// there is no external object 'really' attached.
	return
}
