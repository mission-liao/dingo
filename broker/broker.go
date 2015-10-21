package broker

import (
	"github.com/mission-liao/dingo/meta"
)

//
type Broker interface {
	Producer
	Consumer
}

//
type Producer interface {

	//
	Send(meta.Task) error
}

//
type Consumer interface {

	// receive task from brokers
	//
	// - tasks: 'dingo' would consume from this channel for new tasks
	// - errs: 'dingo' would consume from this channel for error messages
	// - err: any error during initialization
	Consume(rcpt <-chan Receipt) (tasks <-chan meta.Task, errs <-chan error, err error)

	//
	Stop() (err error)
}

var Status = struct {
	OK               int
	NOK              int
	WORKER_NOT_FOUND int
}{
	1, 2, 3,
}

type Receipt struct {
	Id      string
	Status  int
	Payload interface{}
}

func New(name string, cfg *Config) (b Broker, err error) {
	switch name {
	case "local":
		b = newLocal(cfg)
	}

	return
}
