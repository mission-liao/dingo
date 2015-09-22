package backend

import (
	"../internal/share"
	"../task"
)

type Backend interface {
	share.Server

	Update(r task.Report) error
	Poll(t task.Task, last task.Report) (task.Report, error)
}
