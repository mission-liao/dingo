package dingo

// exporting hooks of external objects(backend, broker
// to internal object(workers, mappers).
//
// instead of exposing the whole 'bridge' interface to internal objects,
// just exposing 'exHooks' to them.
type exHooks interface {
	ReporterHook(eventID int, payload interface{}) (err error)
	StoreHook(eventID int, payload interface{}) (err error)
	ProducerHook(eventID int, payload interface{}) (err error)
	ConsumerHook(eventID int, payload interface{}) (err error)
}

type bridge interface {
	exHooks

	Close() (err error)
	Events() ([]<-chan *Event, error)

	//
	// proxy for Producer
	//
	SendTask(t *Task) (err error)

	//
	// proxy for Consumer
	//
	AddNamedListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan *Task, err error)
	AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *Task, err error)
	StopAllListeners() (err error)

	//
	// proxy for Reporter
	//
	Report(name string, reports <-chan *Report) (err error)

	//
	// proxy for Store
	//
	Poll(t *Task) (reports <-chan *Report, err error)

	//
	// setter
	//
	AttachReporter(r Reporter) (err error)
	AttachStore(s Store) (err error)
	AttachProducer(p Producer) (err error)
	AttachConsumer(c Consumer, nc NamedConsumer) (err error)

	//
	// existence checker
	//
	Exists(it int) bool
}

func newBridge(which string, trans *fnMgr, args ...interface{}) bridge {
	switch which {
	case "local":
		return newLocalBridge(args...)
	case "remote":
		return newRemoteBridge(trans)
	default:
		return newRemoteBridge(trans)
	}

	return nil
}
