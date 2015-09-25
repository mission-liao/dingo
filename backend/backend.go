package backend

import (
	"../internal/share"
	"../task"
)

type Backend interface {
	share.Server

	// send report to backend
	Update(r task.Report) error

	// create a poller to receive reports from backend
	//
	// - multiple poller could be initiated for higher throughput.
	NewPoller(chan<- task.Report) error

	// Monitor for task result, poller should maintain a list of 'checks'.
	Check(t task.Task) error

	// Stop monitoring that task
	//
	// - ID of task / report are the same, therefore we use report here to
	//   get ID of corresponding task.
	Uncheck(r task.Report) error
}

func NewBackend() Backend {
	return &_amqp{}
}
