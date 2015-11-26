package transport

type Report struct {
	I string
	N string
	S int16
	E *Error
	R []interface{}
}

var Status = struct {
	None     int16
	Sent     int16
	Progress int16
	Done     int16
	Fail     int16
	Count    int16 // this field should always the last one

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

//
// getter
//
func (r *Report) ID() string            { return r.I }
func (r *Report) Name() string          { return r.N }
func (r *Report) Status() int16         { return r.S }
func (r *Report) Err() *Error           { return r.E }
func (r *Report) Return() []interface{} { return r.R }

//
// setter
//
func (r *Report) SetReturn(ret []interface{}) { r.R = ret }

//
// checker
//
func (r *Report) Valid() bool { return r.S == Status.None }
func (r *Report) Done() bool  { return r.S == Status.Done || r.S == Status.Fail }
func (r *Report) Equal(other *Report) bool {
	if other == nil {
		return false
	}
	return r.S == other.S
}
