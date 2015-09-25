package task

import (
	"encoding/json"
)

type Task interface {
	GetName() string
	GetId() string
	GetArgs() []interface{}
	ComposeReport(int, error, []interface{}) (Report, error)
}

//
// building block of a task
//
type _task struct {
	Id   string // dingo-generated id
	Name string // function name
	Args []interface{}
}

//
// Task interface
//
func (t *_task) GetName() string      { return t.Name }
func (t *_task) GetId() string        { return t.Id }
func (t *_task) GetArgs() interface{} { return t.Args }
func (t *_task) ComposeReport(s int, err error, r []interface{}) (Report, error) {
	return &_report{
		Id:     t.Id,
		Status: s,
		Err:    err,
		Ret:    r,
	}, nil
}

func UnmarshalTask(buf []byte) (t Task, err error) {
	var _t _task
	err = json.Unmarshal(&_t)
	t = &_t
	return
}
