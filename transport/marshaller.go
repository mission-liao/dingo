package transport

var Encode = struct {
	Default int16
	JSON    int16
	GOB     int16
}{
	0, 1, 2,
}

// requirement:
// - each function should be thread-safe
type Marshaller interface {
	Prepare(name string, fn interface{}) (err error)
	EncodeTask(fn interface{}, task *Task) (b []byte, err error)
	DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error)
	EncodeReport(fn interface{}, report *Report) (b []byte, err error)
	DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error)
}
