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

//
// interface Err
//
func (me *_error) GetCode() int        { return me.Code }
func (me *_error) GetMsg() string      { return me.Msg }
func (me *_error) ComposeError() error { return errors.New(me.Error()) }

// implement interface of error
func (me *_error) Error() string {
	return fmt.Sprintf("[%d] %v", me.Code, me.Msg)
}
