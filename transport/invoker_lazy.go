package transport

import (
	"errors"
	"fmt"
	"reflect"
)

type LazyInvoker struct{}

//
// Invoker interface
//

func (vk *LazyInvoker) Call(f interface{}, param []interface{}) ([]interface{}, error) {
	funcT := reflect.TypeOf(f)

	// make sure parameter matched
	if len(param) != funcT.NumIn() {
		return nil, errors.New(fmt.Sprintf("Parameter Count mismatch: %v %v", len(param), funcT.NumIn()))
	}

	// compose input parameter
	var in = make([]reflect.Value, funcT.NumIn())
	for i := 0; i < funcT.NumIn(); i++ {
		v := reflect.ValueOf(param[i])

		// special handle for pointer
		t := funcT.In(i)
		if t.Kind() == reflect.Ptr {
			v_ := reflect.New(t)
			r_ := v_ // cache the value corresponding to required type
			if v.IsValid() {
				for t.Kind() == reflect.Ptr {
					t = t.Elem()
					v_.Elem().Set(reflect.New(t))
					v_ = v_.Elem()
				}
				for v.Kind() == reflect.Ptr {
					v = v.Elem()
				}
				v_.Elem().Set(v)
			}
			in[i] = r_.Elem()
		} else {
			in[i] = v
		}
	}

	// invoke the function
	ret := reflect.ValueOf(f).Call(in)
	out := make([]interface{}, 0, len(ret))

	for k, v := range ret {
		if v.CanInterface() {
			out = append(out, v.Interface())
		} else {
			return nil, errors.New(fmt.Sprintf("Unable to convert to interface{} for %d, %v", k, v))
		}
	}

	return out, nil
}

func (vk *LazyInvoker) Return(f interface{}, returns []interface{}) ([]interface{}, error) {
	return returns, nil
}
