package dingo

import (
	"encoding/json"
	"fmt"
	"reflect"
)

/*
 Different from JsonMarshaller, which marshal []interface{} to a single byte stream.
 JSONSafeCodec would marshal each element in []interface{} to separated byte steam,
 and unmarshall them to variable with "more accurated" type.

 Note: this Codec can be used with GenericInvoker and LazyInvoker
*/

type JSONSafeCodec struct{}

func (codec *JSONSafeCodec) Prepare(name string, fn interface{}) (err error) {
	return
}

func (codec *JSONSafeCodec) EncodeArgument(fn interface{}, val []interface{}) ([][]byte, error) {
	if len(val) != reflect.TypeOf(fn).NumIn() {
		return nil, fmt.Errorf("Unable to encode %v, because its count of args is wrong", fn)
	}

	return codec.encode(val)
}

func (codec *JSONSafeCodec) DecodeArgument(fn interface{}, bs [][]byte) ([]interface{}, error) {
	funcT := reflect.TypeOf(fn)
	if len(bs) != funcT.NumIn() {
		return nil, fmt.Errorf("Unable to decode %v, because its count of args is wrong", fn)
	}

	return codec.decode(bs, func(i int) reflect.Type {
		return funcT.In(i)
	})
}

func (codec *JSONSafeCodec) EncodeReturn(fn interface{}, val []interface{}) ([][]byte, error) {
	if len(val) != reflect.TypeOf(fn).NumOut() {
		return nil, fmt.Errorf("Unable to encode %v, because its count of args is wrong", fn)
	}

	return codec.encode(val)
}

func (codec *JSONSafeCodec) DecodeReturn(fn interface{}, bs [][]byte) ([]interface{}, error) {
	funcT := reflect.TypeOf(fn)
	if len(bs) != funcT.NumOut() {
		return nil, fmt.Errorf("Unable to decode %v, because its count of args is wrong", fn)
	}

	return codec.decode(bs, func(i int) reflect.Type {
		return funcT.Out(i)
	})
}

func (codec *JSONSafeCodec) encode(vs []interface{}) (bs [][]byte, err error) {
	bs = make([][]byte, 0, len(vs))
	for _, v := range vs {
		var b []byte
		b, err = json.Marshal(v)
		if err != nil {
			return
		}
		bs = append(bs, b)
	}

	return
}

func (codec *JSONSafeCodec) decode(bs [][]byte, tfn func(i int) reflect.Type) ([]interface{}, error) {
	vs := make([]interface{}, 0, len(bs))
	for k, b := range bs {
		t := tfn(k)
		v := reflect.New(t)
		r := v.Elem() // cache the value for the right type
		if r.CanInterface() == false {
			return nil, fmt.Errorf("can't interface of r %d:%v", k, t)
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
