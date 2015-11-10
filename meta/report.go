package meta

type Report struct {
	I string
	N string
	S int
	E *Error
	R []interface{}
}

var Status = struct {
	None     int
	Sent     int
	Progress int
	Done     int
	Fail     int
	Count    int // this field should always the last one

	// these fields are for test
	Test1 int
	Test2 int
	Test3 int
	Test4 int
	Test5 int
	Test6 int
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
func (r *Report) Status() int           { return r.S }
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
