package dingo

import (
	"github.com/mission-liao/dingo/transport"
)

// exporting hooks of external objects(backend, broker
// to internal object(workers, mappers).
//
// instead of exposing the whole 'bridge' interface to internal objects,
// just exposing 'exHooks' to them.
type exHooks interface {
	ReporterHook(eventID int, payload interface{}) (err error)
	ProducerHook(eventID int, payload interface{}) (err error)
}

type bridge interface {
	exHooks

	Close() (err error)
	Events() ([]<-chan *Event, error)

	//
	// proxy for Producer
	//
	SendTask(t *transport.Task) (err error)

	//
	// proxy for Consumer
	//
	AddNamedListener(name string, receipts <-chan *TaskReceipt) (tasks <-chan *transport.Task, err error)
	AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan *transport.Task, err error)
	StopAllListeners() (err error)

	//
	// proxy for Reporter
	//
	Report(reports <-chan *transport.Report) (err error)

	//
	// proxy for Store
	//
	Poll(t *transport.Task) (reports <-chan *transport.Report, err error)

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

func newBridge(which string, trans *transport.Mgr, args ...interface{}) bridge {
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
