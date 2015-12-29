##Backend
> [Next: Function Invoker](invoker.md)

__Backend__ is the interface defined to interact with data stores(ex: Redis) which acts storage of reports, it's composed of two parts:
 - [Reporter](backend.md#reporter)
 - [Store](backend.md#store)

###Reporter
> refer to [GoDoc](https://godoc.org/github.com/mission-liao/dingo#Reporter) for its definition

__Worker__ relies on this interface to send reports to data stores(ex: Redis). Instead of being synchronous, __Worker__ and this interface work in parellel by providing an output channel of __[]byte__. Two things to note:
 - __Dingo__ would make sure that all reports from one task would be sent through the same channel.
 - __Dingo__ would acquire mulitple report channels by calling this method multiple times.
```go
Report(reports <-chan *ReportEnvelope) (id int, err error)
```
The evenlope contains two parts:
 - meta info(ID, name of reports) of this byte stream, so you can rely on it to send this byte stream to correct table.
 - payload: the byte stream to be sent

###Store
> refer to [GoDoc](https://godoc.org/github.com/mission-liao/dingo#Store) for its definition

__Caller__ relies on this interface to poll reports from data stores(ex: Redis). Instead of being synchronous, __Caller__ and this interface work in parellel by getting channels.
```go
Poll(meta Meta) (reports <-chan []byte, err error)
Done(meta Meta) error
```
All reports matching the info provided by [Meta](https://godoc.org/github.com/mission-liao/dingo#Meta) should be sent through this channel. Once more reports would be sent, __Dingo__ would notify this component by call its __Done__ method.
