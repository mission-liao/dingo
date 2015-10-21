package meta

import (
	"errors"
	"fmt"
)

type _error struct {
	Code int
	Msg  string
}

type Err interface {
	GetCode() int
	GetMsg() string
	ComposeError() error
}

func NewErr(code int, err error) Err {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return &_error{
		Code: code,
		Msg:  msg,
	}
}

func NoErr() Err {
	return &_error{}
}

func (me *_error) GetCode() int   { return me.Code }
func (me *_error) GetMsg() string { return me.Msg }
func (me *_error) ComposeError() error {
	return errors.New(fmt.Sprintf("[%d] %v", me.Code, me.Msg))
}
