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
	// create a new consumer to receive tasks
	//
	// parameters:
	// - rcpt: a channel that 'dingo' would send 'Receipt' for tasks from 'tasks'.
	// returns:
	// - tasks: 'dingo' would consume from this channel for new tasks
	// - errs: 'dingo' would consume from this channel for error messages
	// - err: any error during initialization
	AddListener(rcpt <-chan Receipt) (tasks <-chan meta.Task, errs <-chan error, err error)

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
		b, err = newLocal(cfg)
	case "amqp":
		b, err = newAmqp(cfg)
	case "redis":
		b, err = newRedis(cfg)
	}

	return
}
