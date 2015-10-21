package backend

import (
	"github.com/mission-liao/dingo/meta"
)

type Backend interface {
	Reporter
	Store
}

// write reports to backend
type Reporter interface {
	// send report to backend
	//
	// parameters:
	// - report: a input channel to receive report to upload
	// returns:
	// - id: id of 'report', you can used it for later operation.
	// - err: errors
	Report(report <-chan meta.Report) (id string, err error)

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
	Subscribe() (reports <-chan meta.Report, err error)

	// polling results for tasks, callers maintain a list of 'to-check'.
	Poll(id meta.IDer) error

	// Stop monitoring that task
	//
	// - ID of task / report are the same, therefore we use report here to
	//   get ID of corresponding task.
	Done(id meta.IDer) error
}

func New(name string, cfg *Config) (b Backend, err error) {
	switch name {
	case "local":
		b = newLocal(cfg)
	}

	return
}
