package dingo

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

/*
 This Invoker is a generic one which can convert values from different types.
 For example:
  - a struct can be converted from a map or another struct.
  - pointers or pointers of pointers or ... is also handled.
 However, the flexibility comes with price, it's also the slowest option.
 All builtin Marshaller(s) could be used with this Invoker.
*/
type GenericInvoker struct{}

func (vk *GenericInvoker) convert2slice(v, r reflect.Value, rt reflect.Type) (err error) {
	r.Set(reflect.MakeSlice(rt, 0, v.Len()))
	for i := 0; i < v.Len(); i++ {
		if converted, err_ := vk.convert(v.Index(i), rt.Elem()); err_ != nil {
			err = err_
			break
		} else {
			r.Set(reflect.Append(r, converted))
		}
	}

	return
}

func (vk *GenericInvoker) convert2map(v, r reflect.Value, rt reflect.Type) (err error) {
	// init map
	r.Set(reflect.MakeMap(rt))

	keys := v.MapKeys()
	for _, k := range keys {
		if converted, err_ := vk.convert(v.MapIndex(k), rt.Elem()); err_ != nil {
			// propagate error
			err = err_
			break
		} else {
			r.SetMapIndex(k, converted)
		}
	}

	return
}

func (vk *GenericInvoker) convert2struct(v, r reflect.Value, rt reflect.Type) (err error) {
	var (
		fv, mv reflect.Value
		ft     reflect.StructField
		key    string
	)
	kind := v.Kind()

	if !(kind == reflect.Map || kind == reflect.Struct) {
		err = fmt.Errorf("Only Map/Struct not %v convertible to struct", v.Kind().String())
		return
	}
	for i := 0; i < r.NumField(); i++ {
		if fv = r.Field(i); !fv.CanSet() {
			continue
		}

		var converted reflect.Value

		ft = rt.Field(i)
		switch kind {
		case reflect.Map:
			if ft.Anonymous {
				converted, err = vk.convert(v, ft.Type)
				break
			}
			// json tags
			if key = ft.Tag.Get("json"); key != "" {
				key = strings.Trim(key, "\"")
				keys := strings.Split(key, ",")
				key = keys[0]
				if key == "-" {
					key = ""
				}
			}
			if key == "" {
				key = ft.Name
			}
			if mv = v.MapIndex(reflect.ValueOf(key)); !mv.IsValid() {
				err = fmt.Errorf("Invalid value returned from map by key: %v", ft)
				break
			}

			converted, err = vk.convert(mv, fv.Type())
		case reflect.Struct:
			converted = v.FieldByName(ft.Name)
		}

		if err != nil {
			return err
		}

		fv.Set(converted)
	}

	return err
}

func (vk *GenericInvoker) convert(v reflect.Value, t reflect.Type) (reflect.Value, error) {
	var (
		err error
		val interface{}
	)

	if v.IsValid() {
		if v.Type().Kind() == reflect.Interface {
			// type assertion
			// by convert to interface{} and reflect it
			if val = v.Interface(); val == nil {
				if t.Kind() != reflect.Ptr {
					err = errors.New("Can't pass nil for non-ptr parameter")
				}
				// for pointer type, reflect.Zero create a nil pointer
				return reflect.Zero(t), err
			}

			v = reflect.ValueOf(val)
		}
		if v.Type().ConvertibleTo(t) {
			return v.Convert(t), nil
		}
	}

	deref := 0
	// only reflect.Value from reflect.New is settable
	// reflect.Zero is not.
	ret := reflect.New(t)
	for ; t.Kind() == reflect.Ptr; deref++ {
		t = t.Elem()
		ret.Elem().Set(reflect.New(t))
		ret = ret.Elem()
	}
	elm := ret.Elem()

	switch elm.Kind() {
	case reflect.Struct:
		err = vk.convert2struct(v, elm, t)
	case reflect.Map:
		err = vk.convert2map(v, elm, t)
	case reflect.Slice:
		// TODO: special case for []byte
		err = vk.convert2slice(v, elm, t)
	default:
		if v.Type().ConvertibleTo(t) {
			// gob/json encoding can't handle pointer to value.
			// pointer, or pointer to value would both be converted to value.
			//
			// the underlying type of those parameter with 'Ptr' type would be deduced here.
			elm.Set(v.Convert(t))
		} else {
			err = fmt.Errorf("Unsupported Element Type: %v", elm.Kind().String())
		}
	}

	if deref == 0 {
		ret = ret.Elem()
	} else {
		// derefencing to correct type
		for deref--; deref > 0; deref-- {
			ret = ret.Addr()
		}
	}

	return ret, err
}

func (vk *GenericInvoker) from(val interface{}, t reflect.Type) (reflect.Value, error) {
	if val == nil {
		var err error

		if t.Kind() != reflect.Ptr {
			err = errors.New("Can't pass nil for non-ptr parameter")
		}
		// for pointer type, reflect.Zero create a nil pointer
		return reflect.Zero(t), err
	}

	return vk.convert(reflect.ValueOf(val), t)
}

func (vk *GenericInvoker) Call(f interface{}, param []interface{}) ([]interface{}, error) {
	//
	// reference implementation
	//	  https://github.com/codegangsta/inject/blob/master/inject.go
	//

	var err error

	funcT := reflect.TypeOf(f)

	// make sure parameter matched
	if len(param) != funcT.NumIn() {
		return nil, fmt.Errorf("Parameter Count mismatch: %v %v", len(param), funcT.NumIn())
	}

	// convert param into []reflect.Value
	var in = make([]reflect.Value, funcT.NumIn())
	for i := 0; i < funcT.NumIn(); i++ {
		if in[i], err = vk.from(param[i], funcT.In(i)); err != nil {
			return nil, err
		}
	}

	// invoke the function
	ret := reflect.ValueOf(f).Call(in)
	out := make([]interface{}, funcT.NumOut())

	// we don't have to check the count of return values and out,
	// if not match, it won't build

	for i := 0; i < funcT.NumOut(); i++ {
		if ret[i].CanInterface() {
			out[i] = ret[i].Interface()
		} else {
			return nil, fmt.Errorf("Unable to convert to interface{} for %d", i)
		}
	}

	return out, nil
}

func (vk *GenericInvoker) Return(f interface{}, returns []interface{}) ([]interface{}, error) {
	funcT := reflect.TypeOf(f)

	// make sure parameter matched
	if len(returns) != funcT.NumOut() {
		return nil, fmt.Errorf("Parameter Count mismatch: %v %v", len(returns), funcT.NumOut())
	}

	var (
		err error
		ret reflect.Value
	)

	out := make([]interface{}, funcT.NumOut())
	for i := 0; i < funcT.NumOut(); i++ {
		ret, err = vk.from(returns[i], funcT.Out(i))
		if err != nil {
			return nil, err
		}
		if ret.CanInterface() {
			out[i] = ret.Interface()
		} else {
			return nil, fmt.Errorf("Unable to convert to interface{} for %d", i)
		}
	}

	return out, nil
}
