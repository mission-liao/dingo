package meta

import (
	"fmt"
)

type Error struct {
	Code int
	Msg  string
}

func NewErr(code int, err error) *Error {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return &Error{
		Code: code,
		Msg:  msg,
	}
}

func NoErr() *Error {
	return &Error{}
}

// implement interface of error
func (me *Error) Error() string {
	return fmt.Sprintf("[%d] %v", me.Code, me.Msg)
}
