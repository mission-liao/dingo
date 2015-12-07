package transport

import (
	"reflect"

	"github.com/satori/go.uuid"
)

//
// building block of a task
//
type Task struct {
	H *Header
	A []interface{}
}

func ComposeTask(name string, args []interface{}) (*Task, error) {
	return &Task{
		H: NewHeader(uuid.NewV4().String(), name),
		A: args,
	}, nil
}

//
// getter
//
func (t *Task) ID() string          { return t.H.I }
func (t *Task) Name() string        { return t.H.N }
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
		H: t.H,
		P: &reportPayload{
			S: s,
			E: err_,
			R: r,
		},
	}, nil
}
func (t *Task) Equal(other *Task) bool {
	return t.H.N == other.H.N && reflect.DeepEqual(t.A, other.A)
}
