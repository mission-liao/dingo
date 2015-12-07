package transport

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var Encode = struct {
	Default int16
	JSON    int16
	GOB     int16
}{
	0, 1, 2,
}

// requirement:
// - each function should be thread-safe
type Marshaller interface {
	Prepare(name string, fn interface{}) (err error)
	EncodeTask(task *Task) (b []byte, err error)
	DecodeTask(h *Header, b []byte) (task *Task, err error)
	EncodeReport(report *Report) (b []byte, err error)
	DecodeReport(h *Header, b []byte) (report *Report, err error)
}

//
// JSON
//

type JsonMarshaller struct {
}

func (me *JsonMarshaller) Prepare(string, interface{}) (err error) {
	return
}

func (me *JsonMarshaller) EncodeTask(task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// encode header
	bHead, err := task.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	bArgs, err := json.Marshal(task.Args())
	if err != nil {
		return
	}

	b = append(bHead, bArgs...)
	return
}

func (me *JsonMarshaller) DecodeTask(h *Header, b []byte) (task *Task, err error) {
	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// decode payload
	var args []interface{}
	err = json.Unmarshal(b[h.Length():], &args)
	if err == nil {
		task = &Task{
			H: h,
			A: args,
		}
	}

	return
}

func (me *JsonMarshaller) EncodeReport(report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// encode header
	bHead, err := report.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	bPayload, err := json.Marshal(report.P)
	if err != nil {
		return
	}

	b = append(bHead, bPayload...)
	return
}

func (me *JsonMarshaller) DecodeReport(h *Header, b []byte) (report *Report, err error) {
	var payloads reportPayload

	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// decode payload
	err = json.Unmarshal(b[h.Length():], &payloads)
	if err == nil {
		report = &Report{
			H: h,
			P: &payloads,
		}
	}

	return
}

//
// Gob
//

type GobMarshaller struct {
}

// Gob needs to register type before encode/decode
func (me *GobMarshaller) Prepare(name string, fn interface{}) (err error) {
	fT := reflect.TypeOf(fn)
	if fT.Kind() != reflect.Func {
		err = errors.New(fmt.Sprintf("fn is not a function but %v", fn))
		return
	}

	reg := func(v reflect.Value) (err error) {
		if !v.CanInterface() {
			err = errors.New(fmt.Sprintf("Can't convert to value in input of %v for name:%v", fn, name))
			return
		}

		gob.Register(v.Interface())
		return
	}

	for i := 0; i < fT.NumIn(); i++ {
		// create a zero value of the type of parameters
		err = reg(reflect.Zero(fT.In(i)))
		if err != nil {
			return
		}
	}

	for i := 0; i < fT.NumOut(); i++ {
		err = reg(reflect.Zero(fT.Out(i)))
		if err != nil {
			return
		}
	}

	return
}

func (me *GobMarshaller) EncodeTask(task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	// encode header
	bHead, err := task.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	var buff *bytes.Buffer = new(bytes.Buffer)
	err = gob.NewEncoder(buff).Encode(task.Args())
	if err == nil {
		b = append(bHead, buff.Bytes()...)
	}
	return
}

func (me *GobMarshaller) DecodeTask(h *Header, b []byte) (task *Task, err error) {
	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// decode payload
	var args []interface{}
	err = gob.NewDecoder(bytes.NewBuffer(b[h.Length():])).Decode(&args)
	if err == nil {
		task = &Task{
			H: h,
			A: args,
		}
	}
	return
}

func (me *GobMarshaller) EncodeReport(report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	// encode header
	bHead, err := report.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	var buff *bytes.Buffer = new(bytes.Buffer)
	err = gob.NewEncoder(buff).Encode(report.P)
	if err == nil {
		b = append(bHead, buff.Bytes()...)
	}
	return
}

func (me *GobMarshaller) DecodeReport(h *Header, b []byte) (report *Report, err error) {
	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// decode payload
	var payload reportPayload
	err = gob.NewDecoder(bytes.NewBuffer(b[h.Length():])).Decode(&payload)
	if err == nil {
		report = &Report{
			H: h,
			P: &payload,
		}
	}
	return
}
