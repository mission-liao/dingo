package broker

import (
	"../internal/share"
	"../task"
)

//
type Sender interface {

	//
	Send(task.Task) error
}

//
type Receiver interface {

	//
	NewReceiver(tasks chan<- taskInfo, errs chan<- error) (id string, err error)
}

//
type Broker interface {
	Sender
	Receiver
	share.Server
}

type TaskInfo struct {
	T    task.Task
	Done chan<- Receipt
	Id   string
}

type ErrInfo struct {
	Err error
	Id  string
}

var Status = struct {
	OK               int
	WORKER_NOT_FOUND int
}{
	1, 2,
}

type Receipt struct {
	Status  int
	Payload interface{}
}

// TODO: accept configuration
func NewBroker() Broker {
	return &_amqp{}
}
