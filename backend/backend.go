package backend

import (
	"github.com/mission-liao/dingo/transport"
)

type Backend interface {
	Reporter
	Store
}

type Envelope struct {
	ID   transport.Meta
	Body []byte
}

// TODO: add test case for consecutive calling to Reporter.Report

// write reports to backend
type Reporter interface {
	// send report to backend
	//
	// parameters:
	// - reports: a input channel to receive report to upload
	// returns:
	// - err: errors
	Report(reports <-chan *Envelope) (id int, err error)
}

// read reports from backend
type Store interface {
	// polling results for tasks, callers maintain a list of 'to-check'.
	Poll(id transport.Meta) (reports <-chan []byte, err error)

	// TODO: test case, make sure queue is deleted after 'Done'

	// Stop monitoring that task
	//
	// - ID of task / report are the same, therefore we use report here to
	//   get ID of corresponding task.
	Done(id transport.Meta) error
}

func New(name string, cfg *Config) (b Backend, err error) {
	switch name {
	case "local":
		b, err = newLocal(cfg)
	case "amqp":
		b, err = newAmqp(cfg)
	case "redis":
		b, err = newRedis(cfg)
	}

	return
}
