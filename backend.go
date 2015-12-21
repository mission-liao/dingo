package dingo

type Backend interface {
	Reporter
	Store
}

type ReportEnvelope struct {
	ID   Meta
	Body []byte
}

var ReporterEvent = struct {
	BeforeReport int
}{
	1,
}

/*
 Reporter(s) is responsible for sending reports to backend(s). The interaction between
 Reporter(s) and dingo are asynchronous by channels.
*/
type Reporter interface {
	// hook for listening events from dingo
	// parameter:
	// - eventID: which event?
	// - payload: corresponding payload, its type depends on 'eventID'
	// returns:
	// - err: errors
	ReporterHook(eventID int, payload interface{}) (err error)

	// attach a report channel to backend.
	//
	// parameters:
	// - reports: a input channel to receive reports from dingo.
	// returns:
	// - err: errors
	Report(reports <-chan *ReportEnvelope) (id int, err error)
}

/*
 Store(s) is responsible for receiving reports from backend(s)
*/
type Store interface {
	// polling reports for tasks
	//
	// parameters:
	// - meta: the meta info of that task to be polled.
	// returns:
	// - reports: the output channel for dingo to receive reports.
	Poll(meta Meta) (reports <-chan []byte, err error)

	// Stop monitoring that task
	//
	// parameters:
	// - id the meta info of that task/report to stop polling.
	Done(meta Meta) error
}
