##Troubleshooting
> [Next: Broker](broker.md)

Most APIs in this library are executed asynchronously in separated go-routines. Therefore, we can't report a failure by returning an error. Instead, an event channel could be subscribed for failure events.
```go
// subscribe a event channel
_, events, err := app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
// initiate a go routine to log events
go func () {
  for {
    select e, ok := <-events:
      if !ok {
        // dingo is closed
      }
      // log it
      fmt.Printf("%v\n", e)
  }
}()
```

You can also turn on the option:__Progress Reports__ to receive more detailed execution reports.
```go
reports, err := app.Call("YourTask", dingo.DefaultOption().MonitorProgress(true), arg1, arg2, ...)
<-reports // the first report means the task is received by workers
<-reports // the second report means the task is sent to worker functions
<-reports // the final report is either success with return values or failure with error info
```

Besides these, I would like to add more utility/mechanism to help troubleshooting, if anyone have any suggestion, please feel free to raise an issue for discussion.
