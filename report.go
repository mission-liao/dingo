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

/*ID refers to identifier of this report, and all reports belongs to one task share the same ID.
 */
func (r *Report) ID() string { return r.H.I }

/*
Name refers to the name of this task that the report belongs to.
*/
func (r *Report) Name() string { return r.H.N }

/*Status refers to current execution status of this task
 */
func (r *Report) Status() int16 { return r.P.S }

/*Error refers to possible error raised during execution, which is packed into
transportable form.
*/
func (r *Report) Error() *Error { return r.P.E }

/*Option refers to execution options of the report, inherits from the tasks that it belongs to.
 */
func (r *Report) Option() *Option { return r.P.O }

/*Return refers to the return values of worker function.
 */
func (r *Report) Return() []interface{} { return r.P.R }

/*Done means the task is finished, and no more reports would be sent. Either Error() or Return() would
have values to check.
*/
func (r *Report) Done() bool { return r.P.S == Status.Success || r.P.S == Status.Fail }

/*OK means the task is done and succeeded, Return() would have values to check.
 */
func (r *Report) OK() bool { return r.P.S == Status.Success }

/*Fail means the task is done and failed, Error() would have values to check.
 */
func (r *Report) Fail() bool { return r.P.S == Status.Fail }

func (r *Report) setReturn(ret []interface{}) { r.P.R = ret }
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
