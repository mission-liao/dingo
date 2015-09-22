package task

type Report interface {
	GetStatus() int
	GetReturn() []interface{}
	GetId() string
	Identical(Report) bool
}

type _report struct {
	Id     int
	Status int
	Ret    []interface{}
}

//
// Report interface
//
func (r *_report) GetStatus() int           { return r.Status }
func (r *_report) GetReturn() []interface{} { return r.Ret }
func (r *_report) GetId() string            { return r.Id }
func (r *_report) Identical(other Report) bool {
	if other == nil {
		return false
	}
	return r.Status == other.GetStatus()
}
