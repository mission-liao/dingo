##Marshaller
> [Next: ID Maker](id_maker.md)

__Dingo__ relies on this interface to marshall [dingo.Task](https://godoc.org/github.com/mission-liao/dingo#Task) and [dingo.Report](https://godoc.org/github.com/mission-liao/dingo#Report) to/from __[]byte__.
```go
Prepare(name string, fn interface{}) (err error)
EncodeTask(fn interface{}, task *Task) (b []byte, err error)
DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error)
EncodeReport(fn interface{}, report *Report) (b []byte, err error)
DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error)
```
When a customized marshalller is required, it's too cumbersome to implement this interface because there are many fields in tasks/reports to take care of. Instead, you can rely on [dingo.CustomMarshaller](https://godoc.org/github.com/mission-liao/dingo#CustomMarshaller) which provides an easier way to provide a custom marshaller. The CustomMarshaller accept an callback interface: [dingo.CustomMarshallerCodec](https://godoc.org/github.com/mission-liao/dingo#CustomMarshallerCodec), which makes you focus on arguments and return values. An example CustomMarshallerCodec by JSON encode:
```
work := func concate(words []string) (ret string) { return }

type testCustomMarshallerCodec struct{}
func (me *testCustomMarshallerCodec) Prepare(name string, fn interface{}) (err error) { return }
func (me *testCustomMarshallerCodec) EncodeArgument(_ interface{}, val []interface{}) (bs [][]byte, err error) {
	b, _ := json.Marshal(val[0])
	bs = [][]byte{b}
	return
}
func (me *testCustomMarshallerCodec) DecodeArgument(_ interface{}, bs [][]byte) (val []interface{}, err error) {
	var input []string
	json.Unmarshal(bs[0], &input)
	val = []interface{}{input}
	return
}
func (me *testCustomMarshallerCodec) EncodeReturn(_ interface{}, val []interface{}) (bs [][]byte, err error) {
	b, _ := json.Marshal(val[0])
	bs = [][]byte{b}
	return
}
func (me *testCustomMarshallerCodec) DecodeReturn(_ interface{}, bs [][]byte) (val []interface{}, err error) {
	var ret string
	json.Unmarshal(bs[0], &ret)
	val = []interface{}{ret}
	return
}

// To test a Marshaller locally, prepare a "remote" mode App, and attach it with local-broker and local-backend
app, _ := dingo.NewApp("remote", nil)
bkd, _ := dingo.NewLocalBackend(nil, nil)
app.Use(bkd, dingo.ObjT.Default)
brk, _ := dingo.NewLocalBroker(nil, nil)
app.Use(brk, dingo.ObjT.Default)

// register worker function
app.Regiser("MyWork", work)
app.AddMarshaller(101, &struct{
    dingo.LazyInvoker
    dingo.CustomMarshaller
}{
    dingo.LazyInvoker{},
    dingo.CustomMarshaller{Codec: &testCustomMarshallerCodec{}},
})
// make dingo use the new marshaller for my worker function
app.SetMarshaller("MyWork, 101)
```
A full runnable example on [GoDoc](https://godoc.org/github.com/mission-liao/dingo#example-CustomMarshallerCodec)
