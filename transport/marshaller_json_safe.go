package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

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
	bArgs, offs, err := me.encode(args)
	if err != nil {
		return
	}
	// option
	bOpt, err := json.Marshal(task.P.O)
	if err != nil {
		return
	}
	// here we assume 'option' is smaller than 'args'
	offs = append(offs, uint64(len(bOpt)))

	// update registries in header
	for _, v := range offs {
		task.H.Append(v)
	}

	// encode header
	bHead, err := task.H.Flush()
	if err != nil {
		return
	}

	b = append(bHead, bArgs...)
	b = append(b, bOpt...)
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

	// decode args
	args, offset, err := me.decode(b[h.Length():], ps[:len(ps)-1], func(i int) reflect.Type {
		return funcT.In(i)
	})
	if err != nil {
		return
	}

	// decode option
	var o *Option
	c := offset + h.Length()
	{
		p := ps[len(ps)-1]
		err = json.Unmarshal(b[c:c+p], &o)
		if err != nil {
			return
		}
		c += p
	}

	task = &Task{
		H: h,
		P: &taskPayload{
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
	bReturns, offs, err := me.encode(returns)
	if err != nil {
		return
	}

	var b_ []byte
	// status
	{
		b_, err = json.Marshal(report.P.S)
		if err != nil {
			return
		}

		offs = append(offs, uint64(len(b_)))
		// TODO: not efficient, should do this at that last
		bReturns = append(bReturns, b_...)
	}

	// err
	{
		b_, err = json.Marshal(report.P.E)
		if err != nil {
			return
		}

		offs = append(offs, uint64(len(b_)))
		// TODO: not efficient, should do this at that last
		bReturns = append(bReturns, b_...)
	}

	// option
	{
		b_, err = json.Marshal(report.P.O)
		if err != nil {
			return
		}

		offs = append(offs, uint64(len(b_)))
		// TODO: not efficient, should do this at that last
		bReturns = append(bReturns, b_...)
	}

	for _, v := range offs {
		report.H.Append(v)
	}

	// encode header
	bHead, err := report.H.Flush()
	if err != nil {
		return
	}

	b = append(bHead, bReturns...)
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

	// decode returns
	returns, offset, err := me.decode(b[h.Length():], ps[:len(ps)-3], func(i int) reflect.Type {
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
	c := offset + h.Length()
	{
		p := ps[len(ps)-3]
		err = json.Unmarshal(b[c:c+p], &s)
		if err != nil {
			return
		}
		c += p
	}

	// decode err
	{
		p := ps[len(ps)-2]
		err = json.Unmarshal(b[c:c+p], &e)
		if err != nil {
			return
		}
		c += p
	}

	// decode option
	{
		p := ps[len(ps)-1]
		err = json.Unmarshal(b[c:c+p], &o)
		if err != nil {
			return
		}
		c += p
	}

	report = &Report{
		H: h,
		P: &reportPayload{
			S: s,
			E: e,
			O: o,
			R: returns,
		},
	}
	return
}

func (me *JsonSafeMarshaller) encode(vs []interface{}) ([]byte, []uint64, error) {
	bs, offs, length := make([][]byte, 0, len(vs)), make([]uint64, 0, len(vs)), uint64(0)
	for _, v := range vs {
		b_, err := json.Marshal(v)
		kk := string(b_)
		fmt.Sprintf("%v", kk)
		if err != nil {
			return nil, []uint64{}, err
		}
		bs = append(bs, b_)
		offs = append(offs, uint64(len(b_)))
		length += uint64(len(b_))
	}

	b := make([]byte, 0, length)
	for _, v := range bs {
		b = append(b, v...)
	}

	return b, offs, nil
}

func (me *JsonSafeMarshaller) decode(b []byte, offs []uint64, tfn func(i int) reflect.Type) ([]interface{}, uint64, error) {
	vs := make([]interface{}, 0, len(offs))
	c := uint64(0)
	for k, o := range offs {
		if c+o > uint64(len(b)) {
			return nil, 0, errors.New(fmt.Sprintf("buffer overrun: %d, %d, %d, %d", k, c, o, len(b)))
		}
		t := tfn(k)
		v := reflect.New(t)
		r := v.Elem() // cache the value for the right type
		if r.CanInterface() == false {
			return nil, 0, errors.New(fmt.Sprintf("can't interface of r %d:%v", k, t))
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
		err := json.Unmarshal(b[c:c+o], v.Interface())
		if err != nil {
			return nil, 0, err
		}

		vs = append(vs, r.Interface())
		c += o
	}

	return vs, c, nil
}
