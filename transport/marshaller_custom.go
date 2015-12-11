package transport

import (
	"encoding/json"
	"errors"
)

/*
 A helper Marshaller for users to create customized Marshaller(s) by providing
 several hooks. Users just need to take care of things they know:
  - input arguments
  - outpu return values
 other payloads of task/report are handled by CustomMarshaller.

 Here is a partial demo with json:
   fn := func(msg string, category int) (done bool) {
     ...
   }

   // register corresponding marshaller
   app.AddMarshaller(mashId, &struct{
     myCustomInvoker
     CustomMarshaller
   }{
     myCustomInvoker{},
     CustomMarshaller{
       Encode: func(output bool, val []interface{}) (bs [][]byte, err error) {
         bs = [][]byte{}
         if output {
           // marshall each argument one by one
           bMsg, _ := json.Marshal(val[0])
           bCategory, _ := json.Marshal(val[1])
           bs = append(bs, bMsg, bCategory)
         } else {
           // marshall return value one by one
           bDone, _ := json.Marshal(val[0])
           bs = append(bs, bDone)
         }
         return
       },
       Decode: func(output bool, bs [][]byte) (val []interface{}, err error) {
         val = []interface{}{}
         if output {
           var (
             msg      string
             category int
           )
           // unmarshall each argument
           json.Unmarshal(bs[0], &msg)
           json.Unmarshal(bs[1], &category)
           val = append(val, msg, category)
         } else {
           var done bool
           // unmarshal each return value
           json.Unmarshal(bs[0], &done)
           val = append(val, done)
         }
         return
       },
     }
   })
*/
type CustomMarshaller struct {
	// when "output is true, "val" refers to return values, otherwise, it's input arguments.
	Encode func(output bool, val []interface{}) ([][]byte, error)

	// when "output is true, "bs" refers byte stream of return values,
	// otherwise, it's from input arguments.
	Decode func(output bool, bs [][]byte) ([]interface{}, error)

	// a hook when Marshaller.Prepare is called, skip this hook if your marshaller
	// doesn't need prepare.
	PrepareHook func(name string, fn interface{}) error
}

func (me *CustomMarshaller) Prepare(name string, fn interface{}) (err error) {
	if me.PrepareHook != nil {
		err = me.PrepareHook(name, fn)
	}

	return
}

func (me *CustomMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	bs, args := [][]byte{}, task.Args()
	if len(args) > 0 {
		if me.Encode == nil {
			err = errors.New("Encode hook is not available")
			return
		}

		bs, err = me.Encode(false, args)
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

func (me *CustomMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
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

	var args []interface{}
	// option would only occupy 1 slot
	if len(bs) > 1 {
		if me.Decode == nil {
			err = errors.New("Decode hook is not available")
			return
		}

		args, err = me.Decode(false, bs[:len(bs)-1])
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

func (me *CustomMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	report.H.Reset()

	bs, returns := [][]byte{}, report.Return()
	if len(returns) > 0 {
		if me.Encode == nil {
			err = errors.New("Encode hook is not available")
			return
		}

		bs, err = me.Encode(true, returns)
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

func (me *CustomMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
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

	var returns []interface{}
	if len(bs) > 3 {
		if me.Decode == nil {
			err = errors.New("Decode hook is not available")
			return
		}

		returns, err = me.Decode(true, bs[:len(bs)-3])
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
