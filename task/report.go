package task

type Report interface {
	//
	// getter
	//

	GetStatus() int
	GetReturn() []interface{}
	GetId() string

	// compare
	Valid() bool
	Identical(Report) bool
	Done() bool
}

type _report struct {
	Id     string
	Status int
	Err    error
	Ret    []interface{}
}

var Status = struct {
	None     int
	Sent     int
	Progress int
	Done     int
	Fail     int
}{
	0, 1, 2, 3, 4,
}

//
// Report interface
//

func (r *_report) GetStatus() int           { return r.Status }
func (r *_report) GetReturn() []interface{} { return r.Ret }
func (r *_report) GetId() string            { return r.Id }
func (r *_report) GetError() error          { return r.Err }
func (r *_report) Valid() bool              { return r.Status == Status.None }
func (r *_report) Identical(other Report) bool {
	if other == nil {
		return false
	}
	return r.Status == other.GetStatus()
}
func (r *_report) Done() bool { return r.Status == Status.Done }
