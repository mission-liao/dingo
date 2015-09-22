package task

type Task interface {
	GetName() string
	GetId() string
	GetArgs() []interface{}
	ComposeReport(int, []interface{}) (Report, error)
}

//
// building block of a task
//
type _task struct {
	Id   string // dingo-generated id
	Name string // function name
	Args []interface{}
}

//
// Task interface
//
func (t *_task) GetName() string      { return t.Name }
func (t *_task) GetId() string        { return t.Id }
func (t *_task) GetArgs() interface{} { return t.Args }
func (t *_task) ComposeReport(s int, r []interface{}) (Report, error) {
	return &_report{
		Status: s,
		Ret:    r,
	}, nil
}
