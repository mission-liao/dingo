##Report
> [Next: TroubleShooting](troubleshooting.md)

Unlike [dingo.Task](https://godoc.org/github.com/mission-liao/dingo#Task), which is invisible to users. [dingo.Report](https://godoc.org/github.com/mission-liao/dingo#Report) is the only way for users to access return values from worker functions. What we provide is a __channel__ feed with *[dingo.Report](https://godoc.org/github.com/mission-liao/dingo#Report) by __Dingo__ and a fancy wrapper to make it easy to use.

You can await a channel in blocking way:
```go
wait := func (reports <-chan *dingo.Report) {
    for {
        r := <-reports
        if r.OK() {
            // get the return values
        }
        if r.Done() {
            break
        }
    }
}
wait(reports)
```
Or raise a go routine to wait asynchronously:
```go
go wait(reports)
```
Or by using [dingo.Result](https://godoc.org/github.com/mission-liao/dingo#Result):
```go
result := dingo.NewResult(app.Call("yourTask", nil, arg1, arg2 ...))
// synchronous waiting
if err := result.Wait(0); err != nil {
  var ret []interface{} = result.Last.Return()
}

// or polling with timeout
for dingo.ResultError.Timeout == result.Wait(1 * time.Second) {
  fmt.Println("waiting.....")
}
if result.OK() {
    var ret []interface{} = result.Last.Return()
    ...
} else if result.NOK() {
} else {
    // if you reach here, fire me a bug
}

// or by handler
result.OnOK(func(_1 retType1, _2 retType2 ...) {
  // no more assertion is required
  // _1, _2 are the right types of your return values.
})
result.OnNOK(func(e1 *dingo.Error, e2 error) {
  // any possible error goes here
})
// then is asynchronous waiting
err = result.Then() // only error when you don't set any handler
```
