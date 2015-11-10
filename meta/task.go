package meta

import (
	"reflect"
)

type ID interface {
	GetId() string
}

//
// building block of a task
//
type Task struct {
	Id   string // dingo-generated id
	Name string // function name
	Args []interface{}
}

//
// Task interface
//
func (t *Task) ComposeReport(s int, r []interface{}, err interface{}) (*Report, error) {
	var err_ *Error
	if err != nil {
		switch v := err.(type) {
		case error:
			err_ = NewErr(0, v)
		case *Error:
			err_ = v
		default:
			// TODO: what? log?
			err_ = nil
		}
	}
	return &Report{
		Id:     t.Id,
		Status: s,
		Err:    err_,
		Name:   t.Name,
		Ret:    r,
	}, nil
}
func (t *Task) Equal(other *Task) bool {
	return t.Name == other.Name && reflect.DeepEqual(t.Args, other.Args)
}
