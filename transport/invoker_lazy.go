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
			in[i] = *vk.toPointer(t, v)
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
	funcT := reflect.TypeOf(f)
	if len(returns) != funcT.NumOut() {
		return nil, errors.New(fmt.Sprintf("Parameter Count mismatch: %v %v", len(returns), funcT.NumOut()))
	}

	for k, v := range returns {
		t := funcT.Out(k)
		// special handle for pointer
		if t.Kind() == reflect.Ptr {
			r := *vk.toPointer(t, reflect.ValueOf(v))
			if r.CanInterface() {
				returns[k] = r.Interface()
			} else {
				return nil, errors.New(fmt.Sprintf("can't interface of %v from %d:%v", r, k, v))
			}
		}
	}

	return returns, nil
}

//
// private function
//

func (vk *LazyInvoker) toPointer(t reflect.Type, v reflect.Value) *reflect.Value {
	if t.Kind() != reflect.Ptr {
		return &v
	}

	v_ := reflect.New(t)
	r_ := v_.Elem()
	if v.IsValid() {
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			if t.Kind() == reflect.Ptr {
				v_.Elem().Set(reflect.New(t))
				v_ = v_.Elem()
			} else {
				if v.Kind() != reflect.Ptr {
					v_.Elem().Set(reflect.New(t))
					v_ = v_.Elem()
				}
			}
		}
		for v.Kind() == reflect.Ptr {
			if v.Elem().Kind() == reflect.Ptr {
				v = v.Elem()
			} else {
				break
			}
		}
		v_.Elem().Set(v)
	}

	return &r_
}
