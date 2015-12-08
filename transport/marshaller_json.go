package transport

import (
	"encoding/json"
	"errors"
)

type JsonMarshaller struct{}

func (me *JsonMarshaller) Prepare(string, interface{}) (err error) {
	return
}

func (me *JsonMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// encode header
	bHead, err := task.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	bArgs, err := json.Marshal(task.Args())
	if err != nil {
		return
	}

	b = append(bHead, bArgs...)
	return
}

func (me *JsonMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// decode payload
	var args []interface{}
	err = json.Unmarshal(b[h.Length():], &args)
	if err == nil {
		task = &Task{
			H: h,
			A: args,
		}
	}

	return
}

func (me *JsonMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// encode header
	bHead, err := report.H.Flush()
	if err != nil {
		return
	}

	// encode payload
	bPayload, err := json.Marshal(report.P)
	if err != nil {
		return
	}

	b = append(bHead, bPayload...)
	return
}

func (me *JsonMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
	var payloads reportPayload

	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// decode payload
	err = json.Unmarshal(b[h.Length():], &payloads)
	if err == nil {
		report = &Report{
			H: h,
			P: &payloads,
		}
	}

	return
}
