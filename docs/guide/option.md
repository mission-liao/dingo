##Execution Option
> [Next: Report](report.md)

You can assign an option for a kind of tasks:
```
app.Register("myTask", func() {})

myOption := dingo.DefaultOption()
app.SetOption("myTask", myOption)
```
Or assign an option to a specific task:
```go
myOption := dingo.DefaultOption()
app.Call("myTask", myOption, ...arguments)
```

These are options that currently available:

###Ignore Report
You don't want to receive any report from this task.
```go
dingo.DefaultConfig().IgnoreReport(true) // default is false
```

###Monitor Progress
You would like to receive more detailed reports from this tasks.
 - Sent: the task is received by __Worker__.
 - Progress: the worker function is about to be invoked.
```go
dingo.DefaultConfig().MonitorProgress(true) // default is false
```
