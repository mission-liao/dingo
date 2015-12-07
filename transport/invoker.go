package transport

type Invoker interface {
	//
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
