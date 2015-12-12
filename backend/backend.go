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

/*
 Reporter(s) is responsible for sending reports to backend(s). The interaction between
 Reporter(s) and dingo are asynchronous by channels.
*/
type Reporter interface {
	// send report to backend
	//
	// parameters:
	// - reports: a input channel to receive reports from dingo.
	// returns:
	// - err: errors
	Report(reports <-chan *Envelope) (id int, err error)
}

/*
 Store(s) is responsible for receiving reports from backend(s)
*/
type Store interface {
	// polling reports for tasks
	//
	// parameters:
	// - id: the meta info of that task to be polled.
	// returns:
	// - reports: the output channel for dingo to receive reports.
	Poll(id transport.Meta) (reports <-chan []byte, err error)

	// Stop monitoring that task
	//
	// parameters:
	// - id the meta info of that task/report to stop polling.
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
