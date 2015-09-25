package broker

import (
	// internal
	"../internal/task"
)

type _direct struct {
}

//
// internal/share.Server interface
//

func (me *_direct) Init() (err error) {
}

func (me *_direct) Close() (err error) {
}

//
// Publisher interface
//

func (me *_direct) Send(t task.Task) (err error) {
}

//
// Consumer interface
//

func (me *_direct) NewReceiver(tasks chan<- taskInfo, errs chan<- errInfo) (string, error) {
}
