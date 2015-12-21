package dingo

/*
 The responsibility of Invoker(s) are:
  - invoking the function by converting []interface{} to []reflect.Value, and calling reflect.Call.
  - convert the return value of reflect.Call from []reflect.Value to []interface{}
 Because most Marshaller(s) can't unmarshal byte stream to original type(s), Invokers are responsible for
 type-correction. The builtin Invoker(s) of "dingo" are also categorized by their ability to correct type(s).

 Note: all implemented functions should be thread-safe.
*/
type Invoker interface {
	// invoker the function "f" by parameters "param", and the returns
	// are represented as []interface{}
	Call(f interface{}, param []interface{}) ([]interface{}, error)

	// when marshal/unmarshal with json, some type information would be lost.
	// this function helps to convert those returns with correct type info provided
	// by function's reflection.
	//
	// parameters:
	// - f: the function
	// - returns: the array of returns
	// returns:
	// converted return array and error
	Return(f interface{}, returns []interface{}) ([]interface{}, error)
}
