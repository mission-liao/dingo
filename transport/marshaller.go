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
	DecodeTask(b []byte) (task *Task, err error)
	EncodeReport(report *Report) (b []byte, err error)
	DecodeReport(b []byte) (report *Report, err error)
}

//
// JSON
//

type jsonMarshaller struct {
}

func (me *jsonMarshaller) Prepare(string, interface{}) (err error) {
	return
}

func (me *jsonMarshaller) EncodeTask(task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	b, err = json.Marshal(task)
	return
}

func (me *jsonMarshaller) DecodeTask(b []byte) (task *Task, err error) {
	var t Task
	err = json.Unmarshal(b, &t)
	if err == nil {
		task = &t
	}
	return
}

func (me *jsonMarshaller) EncodeReport(report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	b, err = json.Marshal(report)
	return
}

func (me *jsonMarshaller) DecodeReport(b []byte) (report *Report, err error) {
	var r Report
	err = json.Unmarshal(b, &r)
	if err == nil {
		report = &r
	}
	return
}

//
// Gob
//

type gobMarshaller struct {
}

// Gob needs to register type before encode/decode
func (me *gobMarshaller) Prepare(name string, fn interface{}) (err error) {
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

func (me *gobMarshaller) EncodeTask(task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	var buff *bytes.Buffer = new(bytes.Buffer)

	enc := gob.NewEncoder(buff)
	err = enc.Encode(task)
	if err == nil {
		b = buff.Bytes()
	}
	return
}

func (me *gobMarshaller) DecodeTask(b []byte) (task *Task, err error) {
	var (
		buff *bytes.Buffer = bytes.NewBuffer(b)
		t    Task
	)

	dec := gob.NewDecoder(buff)
	err = dec.Decode(&t)
	if err == nil {
		task = &t
	}
	return
}

func (me *gobMarshaller) EncodeReport(report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}
	var buff *bytes.Buffer = new(bytes.Buffer)

	enc := gob.NewEncoder(buff)
	err = enc.Encode(report)
	if err == nil {
		b = buff.Bytes()
	}
	return
}

func (me *gobMarshaller) DecodeReport(b []byte) (report *Report, err error) {
	var (
		buff *bytes.Buffer = bytes.NewBuffer(b)
		r    Report
	)

	dec := gob.NewDecoder(buff)
	err = dec.Decode(&r)
	if err == nil {
		report = &r
	}
	return
}
