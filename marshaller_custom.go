package dingo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

/*ComposeBytes composes slice of byte arrays could be composed into one byte stream, along with header section.
 */
func ComposeBytes(h *Header, bs [][]byte) (b []byte, err error) {
	h.Reset()

	length := 0
	for _, v := range bs {
		l := len(v)
		length += l
		h.Append(uint64(l))
	}

	bHead, err := h.Flush(uint64(length))
	if err != nil {
		return
	}

	w := bytes.NewBuffer(bHead)
	for _, v := range bs {
		w.Write(v)
	}

	b = w.Bytes()
	return
}

/*DecomposeBytes can be used to decompose byte streams composed by "ComposeByte" into [][]byte
 */
func DecomposeBytes(h *Header, b []byte) (bs [][]byte, err error) {
	ps := h.Registry()
	bs = make([][]byte, 0, len(ps))
	b = b[h.Length():]

	c := uint64(0)
	for k, p := range ps {
		if c+p > uint64(len(b)) {
			err = fmt.Errorf("buffer overrun: %d, %d, %d, %d", k, c, p, len(b))
			return
		}

		bs = append(bs, b[c:c+p])
		c += p
	}

	return
}

/*CustomMarshallerCodec is used by a marshaller developed to help users to
provide a customized marshaller by providing a "codec" to encode/decode arguments/returns.
*/
type CustomMarshallerCodec interface {

	/*
	 A hook called when CustomMarshaller.Prepare is called.
	*/
	Prepare(name string, fn interface{}) (err error)

	/*
	 encode arguments.
	 - fn: function fingerprint
	 - val: slice of arguments

	 You can encode each argument one by one, and compose them into one
	 slice of byte slice. (or anyway you want)
	*/
	EncodeArgument(fn interface{}, val []interface{}) ([][]byte, error)

	/*
	 decode arguments.
	 - fn: function fingerprint
	 - bs: slice of byte slice
	*/
	DecodeArgument(fn interface{}, bs [][]byte) ([]interface{}, error)

	/*
	 encode returns.
	 - fn: function fingerprint
	 - val: slice of returns

	 You can encode each return one by one, and compose them into one
	 slice of byte slice. (or anyway you want)
	*/
	EncodeReturn(fn interface{}, val []interface{}) ([][]byte, error)

	/*
	 decode arguments.
	 - fn: function fingerprint
	 - bs: slice of byte slice
	*/
	DecodeReturn(fn interface{}, bs [][]byte) ([]interface{}, error)
}

/*CustomMarshaller is a helper Marshaller for users to create customized Marshaller(s) by providing
 several hooks. Users just need to take care of things they know:
  - input arguments
  - outpu return values
 other payloads of task/report are handled by CustomMarshaller.

 Here is a partial demo with json:
   // worker function, we are going to provide a custom marshaller
   // without any reflect for it.
   fn := func(msg string, category int) (done bool) {
      ...
   }

   // implement CustomMarshallerCodec interface
   type myCodec struct {}
   // encoding arguments
   func (c *myCodec) EncodeArgument(fn interface{}, val []interface{}) ([][]byte, error) {
      bMsg, _ := json.Marshal(val[0])
      bCategory, _ := json.Marshal(val[1])
      return [][]byte{bMsg, bCategory}, nil
   }
   // encoding returns
   func (c *myCodec) EncodeReturn(fn interface{}, val []interface{}) ([][]byte, error) {
      bDone, _ := json.Marshal(val[0])
      return [][]byte{bDone}, nil
   }
   // decoding arguments
   func (c *myCodec) DecodeArgument(fn interface{}, bs [][]byte) ([]interface{}, error) {
      var (
         msg      string
         category int
      )
      // unmarshall each argument
      json.Unmarshal(bs[0], &msg)
      json.Unmarshal(bs[1], &category)
      return []interface{}{msg, category}, nil
   }
   func (c *myCodec) DecodeReturn(fn interface{}, bs [][]byte) ([]interface{}, error) {
	var done bool
    json.Unmarshal(bs[0], &done)
	return []interface{}{done}, nil
   }

   // register it to dingo.App
   app.AddMarshaller(expectedMashId, &struct{
      CustomMarshaller,
      myCustomInvoker,
   }{
      CustomMarshaller{Codec: &myCodec{}},
      myCustomInvoker{},
   })
*/
type CustomMarshaller struct {
	Codec CustomMarshallerCodec
}

func (ms *CustomMarshaller) Prepare(name string, fn interface{}) (err error) {
	if ms.Codec != nil {
		err = ms.Codec.Prepare(name, fn)
	}

	return
}

func (ms *CustomMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("Task(nil) is not acceptable")
		return
	}

	bs, args := [][]byte{}, task.Args()
	if len(args) > 0 {
		if ms.Codec == nil {
			err = errors.New("Encode hook is not available")
			return
		}

		bs, err = ms.Codec.EncodeArgument(fn, args)
		if err != nil {
			return
		}
	}

	bOpt, err := json.Marshal(task.P.O)
	if err != nil {
		return
	}

	b, err = ComposeBytes(task.H, append(bs, bOpt))
	return
}

func (ms *CustomMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// clean registry when leaving
	defer func() {
		if h != nil {
			h.Reset()
		}
	}()

	bs, err := DecomposeBytes(h, b)
	if err != nil {
		return
	}

	var args = []interface{}{}
	// option would only occupy 1 slot
	if len(bs) > 1 {
		if ms.Codec == nil {
			err = errors.New("Decode hook is not available")
			return
		}

		args, err = ms.Codec.DecodeArgument(fn, bs[:len(bs)-1])
		if err != nil {
			return
		}
	}

	// decode option
	var o *Option
	err = json.Unmarshal(bs[len(bs)-1], &o)
	task = &Task{
		H: h,
		P: &TaskPayload{
			O: o,
			A: args,
		},
	}
	return
}

func (ms *CustomMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("Report(nil) is not acceptable")
		return
	}

	// reset registry
	report.H.Reset()

	bs, returns := [][]byte{}, report.Return()
	if len(returns) > 0 {
		if ms.Codec == nil {
			err = errors.New("Encode hook is not available")
			return
		}

		bs, err = ms.Codec.EncodeReturn(fn, returns)
		if err != nil {
			return
		}
	}

	bStatus, err := json.Marshal(report.P.S)
	if err != nil {
		return
	}

	bErr, err := json.Marshal(report.P.E)
	if err != nil {
		return
	}

	bOpt, err := json.Marshal(report.P.O)
	if err != nil {
		return
	}

	b, err = ComposeBytes(report.H, append(bs, bStatus, bErr, bOpt))
	return
}

func (ms *CustomMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// clean registry when leaving
	defer func() {
		if h != nil {
			h.Reset()
		}
	}()

	bs, err := DecomposeBytes(h, b)
	if err != nil {
		return
	}

	var returns = []interface{}{}
	if len(bs) > 3 {
		if ms.Codec == nil {
			err = errors.New("Decode hook is not available")
			return
		}

		returns, err = ms.Codec.DecodeReturn(fn, bs[:len(bs)-3])
		if err != nil {
			return
		}
	}

	var (
		s int16
		e *Error
		o *Option
	)

	// decode status
	err = json.Unmarshal(bs[len(bs)-3], &s)
	if err != nil {
		return
	}

	// decode err
	err = json.Unmarshal(bs[len(bs)-2], &e)
	if err != nil {
		return
	}

	// decode option
	err = json.Unmarshal(bs[len(bs)-1], &o)
	if err != nil {
		return
	}

	report = &Report{
		H: h,
		P: &ReportPayload{
			S: s,
			E: e,
			O: o,
			R: returns,
		},
	}
	return
}
