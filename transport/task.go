package transport

import (
	"reflect"

	"github.com/satori/go.uuid"
)

type TaskPayload struct {
	O *Option
	A []interface{}
}

type Task struct {
	H *Header
	P *TaskPayload
}

func ComposeTask(name string, opt *Option, args []interface{}) (*Task, error) {
	if opt == nil {
		opt = NewOption() // make sure it's the default option
	}
	return &Task{
		H: NewHeader(uuid.NewV4().String(), name),
		P: &TaskPayload{
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
func (t *Task) AlmostEqual(other *Task) (same bool) {
	same = t.H.I == other.H.I && t.H.N == other.H.N && reflect.DeepEqual(t.P.O, other.P.O)
	if !same {
		return
	}

	// nil and []interface{}{} should be equal, some marshaller would have different result
	// after 'unmarshalled'.
	if len(t.P.A) == 0 && len(other.P.A) == 0 {
		return
	}

	same = same && reflect.DeepEqual(t.P.A, other.P.A)
	return
}

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
		P: &ReportPayload{
			S: s,
			O: t.P.O,
			E: err_,
			R: r,
		},
	}, nil
}
