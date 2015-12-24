package dingo

/*Broker interface is composed of Producer/Consumer
 */
type Broker interface {
	Producer
	Consumer
}

/*NamedBroker interface is composed of Producer/NamedConsumer
 */
type NamedBroker interface {
	Producer
	NamedConsumer
}

/*ProducerEvent, event IDs that might be passed to dingo.Producer.ProducerHook
 */
var ProducerEvent = struct {
	// a new kind of task is declared.
	DeclareTask int
}{
	1,
}

/*Producer is responsibe for sending tasks to broker(s).
 */
type Producer interface {
	// hook for listening event from dingo
	// parameter:
	// - eventID: which event?
	// - payload: corresponding payload, its type depends on 'eventID'
	// returns:
	// - err: errors
	ProducerHook(eventID int, payload interface{}) (err error)

	// send a task to brokers, it should be a blocking call.
	//
	// parameters:
	// - meta: the meta info of this task to be sent.
	// - b: the byte stream of this task.
	Send(meta Meta, b []byte) error
}

/*ConsumerEvent, IDs of events that might be sent to ConsumerHook
 */
var ConsumerEvent = struct {
}{}

/*Consumer would consume tasks from broker(s). This kind of Consumer(s) is easier
to implement, every task is sent to a single queue, and consumed from a single queue.

The interaction between Consumer(s) and dingo are asynchronous by the channel you provide
in 'AddListener'.
*/
type Consumer interface {
	// hook for listening event from dingo
	// parameter:
	// - eventID: which event?
	// - payload: corresponding payload, its type depends on 'eventID'
	// returns:
	// - err: errors
	ConsumerHook(eventID int, payload interface{}) (err error)

	// create a new listener to receive tasks
	//
	// parameters:
	// - rcpt: a channel that 'dingo' would send 'TaskReceipt' for tasks from 'tasks' channel.
	// returns:
	// - tasks: 'dingo' would consume from this channel for new tasks
	// - err: any error during initialization
	AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan []byte, err error)

	// all listeners are stopped, their corresponding "tasks" channel(returned from AddListener)
	// would be closed.
	StopAllListeners() (err error)
}

/*NamedConsumer would consume tasks from broker(s). Different kind of tasks should be
sent to different queues, and consumed from different queues.

With this kind of Consumer(s), you can deploy different kinds of workers on machines,
and each one of them handles different sets of worker functions.
*/
type NamedConsumer interface {
	// hook for listening event from dingo
	// parameter:
	// - eventID: which event?
	// - payload: corresponding payload, its type depends on 'eventID'
	// returns:
	// - err: errors
	ConsumerHook(eventID int, payload interface{}) (err error)

	// create a new consumer to receive tasks
	//
	// parameters:
	// - name: name of task to be received
	// - rcpt: a channel that 'dingo' would send 'TaskReceipt' for tasks from 'tasks'.
	// returns:
	// - tasks: 'dingo' would consume from this channel for new tasks
	// - err: any error during initialization
	AddListener(name string, rcpt <-chan *TaskReceipt) (tasks <-chan []byte, err error)

	// all listeners are stopped, their corresponding "tasks" channel(returned from AddListener)
	// would be closed.
	StopAllListeners() (err error)
}

/*ReceiptStatus allows broker implementer to know if they have to
reject the received packet or not.
*/
var ReceiptStatus = struct {
	// this task is received successfully.
	OK int
	// something goes wrong
	NOK int
	// dingo can't find workers for this tasks
	WorkerNotFound int
}{
	1, 2, 3,
}

/*TaskReceipt is the receipt allows "dingo" to reject tasks for any reason, the way to handle
rejected tasks are Broker(s) dependent.
*/
type TaskReceipt struct {
	ID      string
	Status  int
	Payload interface{}
}
