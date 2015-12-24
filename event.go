package dingo

import (
	"time"
)

var EventLvl = struct {
	Debug   int
	Info    int
	Warning int
	Error   int
}{
	0,
	1,
	2,
	3,
}

/*
 */
type Event struct {
	// origin of event: please refer to dingo.ObjT for possible values.
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
		Level:   EventLvl.Error,
		Code:    EventCode.Generic,
		Payload: err,
	}
}
