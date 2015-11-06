package common

import (
	"time"
)

var ErrLvl = struct {
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
	Level   int
	Code    int
	Payload interface{}
}

var ErrCode = struct {
	Generic int
}{
	0,
}

func NewEvent(orig, lvl, code int, payload interface{}) *Event {
	return &Event{
		Origin:  orig,
		Time:    time.Now(),
		Level:   lvl,
		Code:    code,
		Payload: payload,
	}
}

func NewEventFromError(orig int, err error) *Event {
	return &Event{
		Origin:  orig,
		Time:    time.Now(),
		Level:   ErrLvl.ERROR,
		Code:    ErrCode.Generic,
		Payload: err,
	}
}
