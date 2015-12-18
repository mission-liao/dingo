package transport

var Encode = struct {
	Default  int
	JSON     int
	GOB      int
	JSONSAFE int
}{
	0, 1, 2, 3,
}

/*
 Marshaller(s) is the major component between []interface{} and []byte.
  - Note: all marshalled []byte should be prefixed with a Header.
  - Note: all implemented functions should be thread-safe.
*/
type Marshaller interface {

	// you can perform any preprocessing for every worker function when registered.
	Prepare(name string, fn interface{}) (err error)

	// Encode a task.
	EncodeTask(fn interface{}, task *Task) (b []byte, err error)

	// Decode a task.
	DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error)

	// Encode a report.
	EncodeReport(fn interface{}, report *Report) (b []byte, err error)

	// Decode a report.
	DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error)
}
