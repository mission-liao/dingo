package task

import (
	"encoding/json"
	"reflect"
)

type IDer interface {
	GetId() string
}

type Task interface {
	IDer
	GetName() string
	GetArgs() []interface{}
	ComposeReport(int, error, []interface{}) (Report, error)
	Equal(t Task) bool
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
func (t *_task) GetName() string        { return t.Name }
func (t *_task) GetId() string          { return t.Id }
func (t *_task) GetArgs() []interface{} { return t.Args }
func (t *_task) ComposeReport(s int, err error, r []interface{}) (Report, error) {
	return &_report{
		Id:     t.Id,
		Status: s,
		Err:    err,
		Ret:    r,
		T:      t,
	}, nil
}
func (t *_task) Equal(other Task) bool {
	return true &&
		t.Name == other.GetName() &&
		reflect.DeepEqual(t.Args, other.GetArgs())
}

func UnmarshalTask(buf []byte) (t Task, err error) {
	var _t _task
	err = json.Unmarshal(buf, &_t)
	if err != nil {
		t = &_t
	}
	return
}
