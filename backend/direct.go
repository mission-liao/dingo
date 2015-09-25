package backend

import (
	// internal
	"../task"
)

type _direct struct {
}

//
// Backend interface
//

func (me *_direct) Update(r task.Report) (err error) {
}

func (me *_direct) Poll(t task.Task)
