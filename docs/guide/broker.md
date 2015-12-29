##Broker
> [Next: Backend](backend.md)

__Broker__ is the interface defined to interact with message brokers(ex: AMQP) which acts as dispatchers in producer-consumer model when deliverying tasks. It's composed of two parts:
 - [Producer](broker.md#producer)
 - [Consumer](broker.md#consumer) or [NamedConsumer](broker.md#namedconsumer)

###Producer
> refer to [GoDoc](https://godoc.org/github.com/mission-liao/dingo#Producer) for its definition

__Caller__ relies on this interface to publish tasks to brokers.

###Consumer
> refer to [GoDoc](https://godoc.org/github.com/mission-liao/dingo#Consumer) for its definition

__Worker__ relies on this interface to consume tasks from brokers. Instead of being synchronous, __Worker__ and this interface work in parellel by exchanging channels.
```go
AddListener(rcpt <-chan *TaskReceipt) (tasks <-chan []byte, err error)
```
__Worker__ would provide a channel to send [receipts](https://godoc.org/github.com/mission-liao/dingo#TaskReceipt) of tasks to __Consumer__. The receipts may contain either one of these status:
 - OK: nothing wrong
 - NOK: something wrong, maybe unmarshalling failed or else.
 - WorkerNotFound: the corresponding worker is not found, please reject this message.

__Consumer__ would provide a channel to send the payloads(__[]byte__) of messages received from message brokers(ex: AMQP).

###NamedConsumer
> refer to [GoDoc](https://godoc.org/github.com/mission-liao/dingo#NamedConsumer) for its definition

In __Consumer__, all tasks would be sent to a single queue on message brokers(ex: AMQP), and the dispatching of tasks would rely on builtin dispatcher of __Dingo__. If it's possible to implement dedicated queues for different tasks, you can choose to implement __NamedConsumer__.
```go
AddListener(name string, rcpt <-chan *TaskReceipt) (tasks <-chan []byte, err error)
```
Different from __Consumer__, a __name__ would be passed when exchanging channels. And in this case, builtin dispatcher of __Dingo__ would not be triggered.
