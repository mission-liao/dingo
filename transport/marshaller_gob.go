package transport

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
)

/*
 Note: this Marshaller can work with both GenericInvoker and LazyInvoker.
*/
type GobMarshaller struct{}

func (me *GobMarshaller) Prepare(name string, fn interface{}) (err error) {
	// Gob needs to register type before encode/decode
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

func (me *GobMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	// reset registry
	task.H.Reset()

	// encode header
	bHead, err := task.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	var buff *bytes.Buffer = new(bytes.Buffer)
	err = gob.NewEncoder(buff).Encode(task.P)
	if err == nil {
		b = append(bHead, buff.Bytes()...)
	}
	return
}

func (me *GobMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
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

	// decode payload
	var payload *TaskPayload
	err = gob.NewDecoder(bytes.NewBuffer(b[h.Length():])).Decode(&payload)
	if err == nil {
		task = &Task{
			H: h,
			P: payload,
		}
	}
	return
}

func (me *GobMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	// reset registry
	report.H.Reset()

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

func (me *GobMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
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

	// decode payload
	var payload *ReportPayload
	err = gob.NewDecoder(bytes.NewBuffer(b[h.Length():])).Decode(&payload)
	if err == nil {
		report = &Report{
			H: h,
			P: payload,
		}
	}
	return
}
