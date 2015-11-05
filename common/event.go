package common

import (
	"time"
)

var ErrLevel = struct {
	DEBUG   int
	INFO    int
	WARNING int
	ERROR   int
}{
	0,
	1,
	2,
	3,
}

type Event struct {
	Origin  int
	Time    time.Time
	Err     error
	Level   int
	Code    int
	Msg     string
	Payload interface{}
}

func NewEvent(orig, lvl, code int, msg string, err error, payload interface{}) *Event {
	return &Event{
		Origin:  orig,
		Time:    time.Now(),
		Err:     err,
		Level:   lvl,
		Code:    code,
		Msg:     msg,
		Payload: payload,
	}
}

func NewEventFromError(orig int, err error) *Event {
	return &Event{
		Origin: orig,
		Time:   time.Now(),
		Level:  ErrLevel.ERROR,
		Err:    err,
	}
}
