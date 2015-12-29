##Caller and Worker
> [Next: Background Worker Pool](background.md)

Below is a flow when you initiate a task and receive reports in __Dingo__.

![Image Of Flow](https://docs.google.com/drawings/d/1Ga6j9gNo-dU7OMe8FLNmwYkRLH_OupWs_10SFJ3igXQ/pub?w=795&h=362)

The key to scaling dividing a big procedure into smaller ones, and making each one of them isolated from each other. For a function in #golang, we can devide it into three parts:
```go
//         passing arguments      receiving return values
//                |                          |
//                v                          v
//       ---------------------   --------------------------
func Foo(count int, msg string) (composed string, err error) {  // |
    return fmt.Sprintf("%v:%v", msg, count), nil                // |<- execution
}                                                               // |
```
- [Passing Arguments](role.md#passing-arguments)
- [Execution](role.md#execution)
- [Receiving Return Values](role.md#receiving-return-values)
- [Example](role.md#example)

###Passing Arguments
There are 3 roles related to this part:
 - __Caller__: the one who...
   - composes arguments into [dingo.Task](https://godoc.org/github.com/mission-liao/dingo#Task)
   - marshall it into __[]byte__
   - sends to brokers
 - __Broker__: the one who dispatching tasks
 - __Worker__: the one who
   - consumes __[]byte__ from brokers
   - unmarshall it into [dingo.Task](https://godoc.org/github.com/mission-liao/dingo#Task)

###Execution
Only __Worker__ relates this part. Upon receiving tasks, __Worker__ should find corresponding functions according to the names of tasks, and execute it.

###Receiving Return Values
After execution:
 - __Worker__:
   - compose return values into [dingo.Report](https://godoc.org/github.com/mission-liao/dingo#Report)
   - marshall it into __[]byte__
   - send it to __Store__.
 - __Caller__:
   - poll from __Store__
   - unmarshall __[]byte__ to [dingo.Report](https://godoc.org/github.com/mission-liao/dingo#Report)
   - dispatch it to the right channel.

###Example
There are runnable examples on GoDoc([Caller](https://godoc.org/github.com/mission-liao/dingo#example-App-Use-Caller) and [Worker](https://godoc.org/github.com/mission-liao/dingo#example-App-Use-Worker)) based on our AMQP adaptor.
