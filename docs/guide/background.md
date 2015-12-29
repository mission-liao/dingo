##Background Worker Pool
> [Next: Configuration](config.md)

__Dingo__ could be used as a worker pool, like those [libraries](https://github.com/avelino/awesome-go#goroutines). Several noticeable differences between this mode to "remote" mode:
 - No database adaptor would be required
 - No more marshalling and unmarshalling
 - Builtin dispatchers for [dingo.Task](https://godoc.org/github.com/mission-liao/dingo#Task) and [dingo.Report](https://godoc.org/github.com/mission-liao/dingo#Report) would be adapted.

An [example](https://godoc.org/github.com/mission-liao/dingo#example-App--Local) on __GoDoc__.
