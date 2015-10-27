package meta

import (
	"encoding/json"
)

type Report interface {
	ID

	//
	// getter
	//

	GetStatus() int
	GetName() string
	GetReturn() []interface{}
	GetErr() Err

	//
	// compare
	//

	Valid() bool
	Identical(Report) bool
	Done() bool

	//
	// setter
	//
	SetReturn([]interface{})
}

type _report struct {
	Id     string
	Name   string
	Status int
	Err_   *_error
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

func (r *_report) GetName() string          { return r.Name }
func (r *_report) GetStatus() int           { return r.Status }
func (r *_report) GetReturn() []interface{} { return r.Ret }
func (r *_report) GetId() string            { return r.Id }
func (r *_report) GetErr() Err              { return r.Err_ }
func (r *_report) Valid() bool              { return r.Status == Status.None }
func (r *_report) Identical(other Report) bool {
	// TODO: seems useless now?
	if other == nil {
		return false
	}
	return r.Status == other.GetStatus()
}
func (r *_report) Done() bool                { return r.Status == Status.Done || r.Status == Status.Fail }
func (r *_report) SetReturn(v []interface{}) { r.Ret = v }

func UnmarshalReport(buf []byte) (r Report, err error) {
	var _r _report
	err = json.Unmarshal(buf, &_r)
	if err == nil {
		r = &_r
	}
	return
}
