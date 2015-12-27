package dingo

import (
	"reflect"
)

type ReportPayload struct {
	S int16
	E *Error
	O *Option
	R []interface{}
}

/*Report is the reports for task execution.

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
	P *ReportPayload
}

var Status = struct {
	None int16

	// the task is sent to the consumer.
	Sent int16

	// the task is sent to workers.
	Progress int16

	// the task is done
	Success int16

	// the task execution is failed.
	Fail int16

	// this field should always the last one
	Count int16
}{
	0, 1, 2, 3, 4, 5,
}

func (r *Report) ID() string    { return r.H.I }
func (r *Report) Name() string  { return r.H.N }
func (r *Report) Status() int16 { return r.P.S }

func (r *Report) Error() *Error               { return r.P.E }
func (r *Report) Option() *Option             { return r.P.O }
func (r *Report) Return() []interface{}       { return r.P.R }
func (r *Report) setReturn(ret []interface{}) { r.P.R = ret }

//
// checker
//

func (r *Report) Done() bool { return r.P.S == Status.Success || r.P.S == Status.Fail }
func (r *Report) OK() bool   { return r.P.S == Status.Success }
func (r *Report) Fail() bool { return r.P.S == Status.Fail }
func (r *Report) almostEqual(other *Report) (same bool) {
	if same = r.H.I == other.H.I &&
		r.H.N == other.H.N &&
		r.P.S == other.P.S &&
		reflect.DeepEqual(r.P.O, other.P.O) &&
		reflect.DeepEqual(r.P.E, other.P.E); !same {
		return
	}

	if len(r.P.R) == 0 && len(other.P.R) == 0 {
		return
	}

	same = same && reflect.DeepEqual(r.P.R, other.P.R)
	return
}
