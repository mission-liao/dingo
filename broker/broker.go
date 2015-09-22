package broker

import (
	"../internal/share"
	"../task"
)

type Publisher interface {
	Send(task.Task) error
}

type Consumer interface {
	Recv(task.Worker) error
}

type Broker interface {
	Publisher
	Consumner
	share.Server
}
