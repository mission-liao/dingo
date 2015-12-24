package dingo

import (
	"reflect"
)

type TaskPayload struct {
	O *Option
	A []interface{}
}

/*Task is the struct records all infomation required for task execution, including:
 - name(type) of task
 - identifier of this task, which should be unique among all tasks of the same name.
 - arguments to be passed into worker function
 - execution option

You don't have to know what it is unless you try to implement:
 - dingo.Marshaller
 - dingo.Invoker
You don't have to create it by yourself, every time you call
dingo.App.Call, one is generated automatically.
*/
type Task struct {
	H *Header
	P *TaskPayload
}

func composeTask(name string, opt *Option, args []interface{}) (t *Task, err error) {
	if opt == nil {
		opt = NewOption() // make sure it's the default option
	}
	id, err := (&uuidMaker{}).NewID()
	if err != nil {
		return
	}

	t = &Task{
		H: NewHeader(id, name),
		P: &TaskPayload{
			O: opt,
			A: args,
		},
	}
	return
}

func (t *Task) ID() string          { return t.H.I }
func (t *Task) Name() string        { return t.H.N }
func (t *Task) Option() *Option     { return t.P.O }
func (t *Task) Args() []interface{} { return t.P.A }
func (t *Task) almostEqual(other *Task) (same bool) {
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
func (t *Task) composeReport(s int16, r []interface{}, err interface{}) (*Report, error) {
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
