package transport

import (
	"reflect"
)

type reportPayload struct {
	S int16
	E *Error
	O *Option
	R []interface{}
}

/*
 Reports for task execution.

 When Report.Done() is true, it means no more reports would be sent.
 And either Report.OK() or Report.Fail() would be true.

 If Report.OK() is true, the task execution is succeeded, and
 you can grab the return value from Report.Returns(). Report.Returns()
 would give you an []interface{}, and the type of every elements in
 that slice would be type-corrected according to the work function.

 If Report.Fail() is true, the task execution is failed, and
 you can grab the reason from Report.Error().
*/
type Report struct {
	H *Header
	P *reportPayload
}

var Status = struct {
	None int16

	// the task is sent to the broker.
	Sent int16

	// the task is sent to workers.
	Progress int16

	// the task is done
	Done int16

	// the task execution is failed.
	Fail int16

	// this field should always the last one
	Count int16

	// TODO: make these private or remove them
	// these fields are for test
	Test1 int16
	Test2 int16
	Test3 int16
	Test4 int16
	Test5 int16
	Test6 int16
}{
	0, 1, 2, 3, 4, 5,

	// for test
	101, 102, 103, 104, 105, 106,
}

func (r *Report) ID() string    { return r.H.I }
func (r *Report) Name() string  { return r.H.N }
func (r *Report) Status() int16 { return r.P.S }

// TODO: renamed to Error()
func (r *Report) Err() *Error                 { return r.P.E }
func (r *Report) Option() *Option             { return r.P.O }
func (r *Report) Return() []interface{}       { return r.P.R }
func (r *Report) SetReturn(ret []interface{}) { r.P.R = ret }

//
// checker
//

// TODO: seems not used, remove it.
func (r *Report) Valid() bool { return r.P.S == Status.None }
func (r *Report) Done() bool  { return r.P.S == Status.Done || r.P.S == Status.Fail }
func (r *Report) Ok() bool    { return r.P.S == Status.Done }
func (r *Report) Fail() bool  { return r.P.S == Status.Fail }
func (r *Report) Equal(other *Report) bool {
	if other == nil {
		return false
	}
	return reflect.DeepEqual(r, other)
}
