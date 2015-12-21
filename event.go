package dingo

import (
	"time"
)

var EventLvl = struct {
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

var EventCode = struct {
	Generic             int
	TaskDeliveryFailure int
	DuplicatedPolling   int
}{
	0, 1, 2,
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
		Level:   EventLvl.ERROR,
		Code:    EventCode.Generic,
		Payload: err,
	}
}
