package broker

// TODO: test multiple consumers with multiple producers

import (
	"github.com/mission-liao/dingo/transport"
)

//
type Broker interface {
	Producer
	Consumer
}

//
type NamedBroker interface {
	Producer
	NamedConsumer
}

//
type Producer interface {
	//
	Send(transport.Meta, []byte) error
}

//
type Consumer interface {
	// create a new consumer to receive tasks
	//
	// parameters:
	// - rcpt: a channel that 'dingo' would send 'Receipt' for tasks from 'tasks'.
	// returns:
	// - tasks: 'dingo' would consume from this channel for new tasks
	// - err: any error during initialization
	AddListener(rcpt <-chan *Receipt) (tasks <-chan []byte, err error)

	//
	StopAllListeners() (err error)
}

type NamedConsumer interface {
	// create a new consumer to receive tasks
	//
	// parameters:
	// - name: name of task to be received
	// - rcpt: a channel that 'dingo' would send 'Receipt' for tasks from 'tasks'.
	// returns:
	// - tasks: 'dingo' would consume from this channel for new tasks
	// - err: any error during initialization
	AddListener(name string, rcpt <-chan *Receipt) (tasks <-chan []byte, err error)
	StopAllListeners() (err error)
}

var Status = struct {
	OK               int
	NOK              int
	WORKER_NOT_FOUND int
}{
	1, 2, 3,
}

type Receipt struct {
	ID      string
	Status  int
	Payload interface{}
}

func NewNamed(name string, cfg *Config) (b NamedBroker, err error) {
	switch name {
	case "amqp":
		b, err = newAmqp(cfg)
	case "redis":
		b, err = newRedis(cfg)
	}

	return
}

func New(name string, cfg *Config) (b Broker, err error) {
	switch name {
	case "local":
		b, err = newLocal(cfg)
	}
	return
}
