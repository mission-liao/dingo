package dingo

/*Backend interface is composed of Reporter/Store
 */
type Backend interface {
	Reporter
	Store
}

/*ReportEvenlope is the standard package sent through the channel to Reporter. The main requirement to fit
is to allow Reporter can know the meta info of the byte stream to send.
*/
type ReportEnvelope struct {
	ID   Meta
	Body []byte
}

/*ReportEvent are those IDs of events that might be sent to ReporterHook
 */
var ReporterEvent = struct {
	// a sequence of reports from this task is about to fire.
	BeforeReport int
}{
	1,
}

/*Reporter is responsible for sending reports to backend(s). The interaction between
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

/*StoreEvent are those IDs of events that might be sent to StoreHook
 */
var StoreEvent = struct {
}{}

/*Store is responsible for receiving reports from backend(s)
 */
type Store interface {
	// hook for listening events from dingo
	// parameter:
	// - eventID: which event?
	// - payload: corresponding payload, its type depends on 'eventID'
	// returns:
	// - err: errors
	StoreHook(eventID int, payload interface{}) (err error)

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
