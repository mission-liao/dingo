package backend

import (
	"github.com/mission-liao/dingo/task"
)

// write reports to backend
type Reporter interface {
	// send report to backend
	//
	// parameters:
	// - report: a input channel to receive report to upload
	// returns:
	// - id: id of 'report', you can used it for later operation.
	// - err: errors
	Report(report <-chan task.Report) (id string, err error)

	// unbind the input channel
	//
	// parameters:
	// - id: id of the input channel, acquired from Reporter.Report
	// returns:
	// - err: errors
	Unbind(id string) (err error)
}

// read reports from backend
type Store interface {
	//
	// - subscribe report channel
	Subscribe() (reports <-chan task.Report, err error)

	// polling results for tasks, callers maintain a list of 'to-check'.
	Poll(id task.IDer) error

	// Stop monitoring that task
	//
	// - ID of task / report are the same, therefore we use report here to
	//   get ID of corresponding task.
	Done(id task.IDer) error
}
