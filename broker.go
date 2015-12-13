package dingo

// TODO: test multiple consumers with multiple producers

import (
	"github.com/mission-liao/dingo/transport"
)

type Broker interface {
	Producer
	Consumer
}

type NamedBroker interface {
	Producer
	NamedConsumer
}

/*
 Producer(s) is responsibe for sending tasks to broker(s).
*/
type Producer interface {
	// send a task to brokers, it should be a blocking call.
	//
	// parameters:
	// - meta: the meta info of this task to be sent.
	// - b: the byte stream of this task.
	Send(meta transport.Meta, b []byte) error
}

/*
 Consumer(s) would consume tasks from broker(s). This kind of Consumer(s) is easier
 to implement, every task is sent to a single queue, and consumed from a single queue.

 The interaction between Consumer(s) and dingo are asynchronous by channels.
*/
type Consumer interface {
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

/*
 Named Consumer(s) would consume tasks from broker(s). Different kind of tasks should be
 sent to different queues, and consumed from different queues.

 With this kind of Consumer(s), you can deploy different kinds of workers on machines,
 and each one of them handles different sets of worker functions.
*/
type NamedConsumer interface {
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

var ReceiptStatus = struct {
	OK               int
	NOK              int
	WORKER_NOT_FOUND int
}{
	1, 2, 3,
}

/*
 Receipt allows "dingo" to reject tasks for any reason, the way to handle
 rejected tasks are Broker(s) dependent.
*/
type TaskReceipt struct {
	ID      string
	Status  int
	Payload interface{}
}
