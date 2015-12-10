package transport

import (
	"reflect"

	"github.com/satori/go.uuid"
)

type taskPayload struct {
	O *Option
	A []interface{}
}

type Task struct {
	H *Header
	P *taskPayload
}

func ComposeTask(name string, opt *Option, args []interface{}) (*Task, error) {
	if opt == nil {
		opt = NewOption() // make sure it's the default option
	}
	return &Task{
		H: NewHeader(uuid.NewV4().String(), name),
		P: &taskPayload{
			O: opt,
			A: args,
		},
	}, nil
}

//
// getter
//
func (t *Task) ID() string             { return t.H.I }
func (t *Task) Name() string           { return t.H.N }
func (t *Task) Option() *Option        { return t.P.O }
func (t *Task) Args() []interface{}    { return t.P.A }
func (t *Task) Equal(other *Task) bool { return reflect.DeepEqual(t, other) }

//
// APIs
//
func (t *Task) ComposeReport(s int16, r []interface{}, err interface{}) (*Report, error) {
	var err_ *Error
	if err != nil {
		switch v := err.(type) {
		case *Error:
			// make sure this type preceding 'error',
			// because *Error implement the 'error' interface.
			err_ = v
		case error:
			err_ = NewErr(0, v)
		default:
			// TODO: what? log?
			err_ = nil
		}
	}
	return &Report{
		H: t.H,
		P: &reportPayload{
			S: s,
			O: t.P.O,
			E: err_,
			R: r,
		},
	}, nil
}
