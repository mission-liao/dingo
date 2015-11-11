package transport

import (
	"fmt"
)

type Error struct {
	C int64
	M string
}

func NewErr(code int64, err error) *Error {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return &Error{
		C: code,
		M: msg,
	}
}

func NoErr() *Error {
	return &Error{}
}

func (me *Error) Code() int64   { return me.C }
func (me *Error) Msg() string   { return me.M }
func (me *Error) Error() string { return fmt.Sprintf("[%d] %v", me.C, me.M) }
