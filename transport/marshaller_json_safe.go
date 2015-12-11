package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

/*
 slice of byte arrays could be composed into one byte stream, along with header section.
*/
func ComposeBytes(h *Header, bs [][]byte) (b []byte, err error) {
	h.Reset()

	length := 0
	for _, v := range bs {
		l := len(v)
		length += l
		h.Append(uint64(l))
	}

	bHead, err := h.Flush()
	if err != nil {
		return
	}

	w := bytes.NewBuffer(make([]byte, 0, length+len(bHead)))
	w.Write(bHead)
	for _, v := range bs {
		w.Write(v)
	}

	b = w.Bytes()
	return
}

/*
 Byte streams composed by "ComposeByte" could be decomposed into [][]byte by
 this function.
*/
func DecomposeBytes(h *Header, b []byte) (bs [][]byte, err error) {
	ps := h.Registry()
	bs = make([][]byte, 0, len(ps))
	b = b[h.Length():]

	c := uint64(0)
	for k, p := range ps {
		if c+p > uint64(len(b)) {
			err = errors.New(fmt.Sprintf("buffer overrun: %d, %d, %d, %d", k, c, p, len(b)))
			return
		}

		bs = append(bs, b[c:c+p])
		c += p
	}

	return
}

/*
 Different from JsonMarshaller, which marshal []interface{} to a single byte stream.
 JsonSafeMarshaller would marshal each element in []interface{} to separated byte steam,
 and unmarshall them to variable with "more accurated" type.

 Note: this Marshaller can be used with GenericInvoker and LazyInvoker
*/
type JsonSafeMarshaller struct{}

func (me *JsonSafeMarshaller) Prepare(string, interface{}) (err error) {
	return
}

func (me *JsonSafeMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	task.H.Reset()

	// encode args
	args, funcT := task.Args(), reflect.TypeOf(fn)
	if len(task.Args()) != funcT.NumIn() {
		err = errors.New(fmt.Sprintf("Unable to encode %v, because its count of args is wrong", task))
		return
	}

	// args
	bs, err := me.encode(args)
	if err != nil {
		return
	}

	// option
	bOpt, err := json.Marshal(task.P.O)
	if err != nil {
		return
	}

	b, err = ComposeBytes(task.H, append(bs, bOpt))
	return
}

func (me *JsonSafeMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
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

	funcT, ps := reflect.TypeOf(fn), h.Registry()
	if funcT.NumIn()+1 != len(ps) {
		err = errors.New(fmt.Sprintf("Unable to decode task, because its count of payload is wrong: %v %v", ps, fn))
		return
	}

	bs, err := DecomposeBytes(h, b)
	if err != nil {
		return
	}

	// decode args
	args, err := me.decode(bs[:len(bs)-1], func(i int) reflect.Type {
		return funcT.In(i)
	})
	if err != nil {
		return
	}

	// decode option
	var o *Option
	{
		err = json.Unmarshal(bs[len(bs)-1], &o)
		if err != nil {
			return
		}
	}

	task = &Task{
		H: h,
		P: &TaskPayload{
			O: o,
			A: args,
		},
	}
	return
}

func (me *JsonSafeMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	report.H.Reset()

	// encode payload
	returns, funcT := report.Return(), reflect.TypeOf(fn)
	if len(returns) != funcT.NumOut() {
		err = errors.New(fmt.Sprintf("Unable to encode %v, because its count of returns is wrong", report))
		return
	}

	// returns
	bs, err := me.encode(returns)
	if err != nil {
		return
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

func (me *JsonSafeMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
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

	funcT, ps := reflect.TypeOf(fn), h.Registry()
	if funcT.NumOut()+3 != len(ps) {
		err = errors.New(fmt.Sprintf("Unable to decode report, because its count of payload is wrong: %v %v", ps, fn))
		return
	}

	bs, err := DecomposeBytes(h, b)
	if err != nil {
		return
	}

	// decode returns
	returns, err := me.decode(bs[:len(bs)-3], func(i int) reflect.Type {
		return funcT.Out(i)
	})
	if err != nil {
		return
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

func (me *JsonSafeMarshaller) encode(vs []interface{}) (bs [][]byte, err error) {
	bs = make([][]byte, 0, len(vs))
	for _, v := range vs {
		var b_ []byte
		b_, err = json.Marshal(v)
		if err != nil {
			return
		}
		bs = append(bs, b_)
	}

	return
}

func (me *JsonSafeMarshaller) decode(bs [][]byte, tfn func(i int) reflect.Type) ([]interface{}, error) {
	vs := make([]interface{}, 0, len(bs))
	for k, b := range bs {
		t := tfn(k)
		v := reflect.New(t)
		r := v.Elem() // cache the value for the right type
		if r.CanInterface() == false {
			return nil, errors.New(fmt.Sprintf("can't interface of r %d:%v", k, t))
		}

		if t.Kind() != reflect.Ptr {
			// inputs for json.Unmarshal can't be nil
			v.Elem().Set(reflect.New(t).Elem())
		} else {
			for t.Kind() == reflect.Ptr {
				t = t.Elem()
				if t.Kind() == reflect.Ptr {
					v.Elem().Set(reflect.New(t))
					v = v.Elem()
				}
			}
		}

		// generate a zero value
		err := json.Unmarshal(b, v.Interface())
		if err != nil {
			return nil, err
		}

		vs = append(vs, r.Interface())
	}

	return vs, nil
}
