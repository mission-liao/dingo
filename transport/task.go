package transport

import (
	"reflect"

	"github.com/satori/go.uuid"
)

//
// building block of a task
//
type Task struct {
	I string // dingo-generated id
	N string // function name
	A []interface{}
}

func ComposeTask(name string, args []interface{}) (*Task, error) {
	return &Task{
		I: uuid.NewV4().String(),
		N: name,
		A: args,
	}, nil
}

//
// getter
//
func (t *Task) ID() string          { return t.I }
func (t *Task) Name() string        { return t.N }
func (t *Task) Args() []interface{} { return t.A }

//
// APIs
//
func (t *Task) ComposeReport(s int16, r []interface{}, err interface{}) (*Report, error) {
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
		I: t.I,
		S: s,
		E: err_,
		N: t.N,
		R: r,
	}, nil
}
func (t *Task) Equal(other *Task) bool {
	return t.N == other.N && reflect.DeepEqual(t.A, other.A)
}
