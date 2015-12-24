package dingo

import (
	"fmt"
	"reflect"
)

/*
 A generic error that could be marshalled/unmarshalled by JSON.
*/
type Error struct {
	C int32
	M string
}

/*
 Error code used in dingo.Error
*/
var ErrCode = struct {
	// the worker function panic
	Panic int32
	// dingo.App shutdown
	Shutdown int32
}{
	1, 2,
}

func NewErr(code int32, err error) *Error {
	if err == nil || reflect.ValueOf(err).IsNil() {
		return &Error{
			C: code,
		}
	}

	return &Error{
		C: code,
		M: err.Error(),
	}
}

func (err *Error) Code() int32   { return err.C }
func (err *Error) Msg() string   { return err.M }
func (err *Error) Error() string { return fmt.Sprintf("[%d] %v", err.C, err.M) }
