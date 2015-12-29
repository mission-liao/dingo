##Function Invoker
> [Next: Marshaller](marshaller.md)

__Dingo__ relies on this interface to invoke functions with (almost) any signature:
```go
Call(f interface{}, param []interface{}) ([]interface{}, error)
Return(f interface{}, returns []interface{}) ([]interface{}, error)
```
Two builtin generic invokers rely on [reflect.Call] to invoke functions, and __Type.In__ to get type info of each argument and apply __type_correction__.

Another method __Return__ is used to apply __type_correction__ for return values after unmarshalling.

If you need to get rid of [reflection] to get performance, you can provide a customized, not generic invoker for your functions. ex:
```go
work := func (n int, name string) {
}

// an invoker for "worker_function"
type myInvoker struct {}
func (me *myInvoker) Call(f interface{}, param []interface{}) (ret []interface{}, err error) {
    // type assertion on everything
    f.(func(int, string))(
        param[0].(int),    // type might be a little bit different based on the marshaller you use.
        param[1].(string), //
    )
    return
}
func (me *myInvoker) Return(f interface{}, ret []interface{}) (fixedRet []interface{}, err error) {
    fixedRet = ret
    return
}

// register your invoker combined with an Marshaller
app.AddMarshaller(101, &struct{
    myInvoker
    dingo.CustomMarshaller
}{
    myInvoker{},
    dingo.CustomMarshaller{Codec: &dingo.JSONSafeCodec{}},
})

// register your worker function
app.Register("MyWork", work)
// use new marshaller+invoker for your worker function
app.SetMarshaller("MyWork", 101)
```
