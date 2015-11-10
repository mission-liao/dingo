package meta

type Report struct {
	Id     string
	Name   string
	Status int
	Err    *Error
	Ret    []interface{}
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
// Report interface
//

func (r *Report) Valid() bool { return r.Status == Status.None }
func (r *Report) Identical(other *Report) bool {
	// TODO: seems useless now?
	if other == nil {
		return false
	}
	return r.Status == other.Status
}
func (r *Report) Done() bool                { return r.Status == Status.Done || r.Status == Status.Fail }
func (r *Report) SetReturn(v []interface{}) { r.Ret = v }
