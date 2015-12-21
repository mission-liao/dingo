package dingo

import (
	"encoding/json"
	"errors"
)

/*
 Note: this Marshaller can only work with GenericInvoker.
*/
type JsonMarshaller struct{}

func (me *JsonMarshaller) Prepare(string, interface{}) (err error) {
	return
}

func (me *JsonMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	task.H.Reset()

	// encode payload
	bPayload, err := json.Marshal(task.P)
	if err != nil {
		return
	}

	// encode header
	bHead, err := task.H.Flush(uint64(len(bPayload)))
	if err != nil {
		return
	}

	b = append(bHead, bPayload...)
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

	// clean registry when leaving
	defer func() {
		if h != nil {
			h.Reset()
		}
	}()

	// decode payload
	var payload *TaskPayload
	err = json.Unmarshal(b[h.Length():], &payload)
	if err == nil {
		task = &Task{
			H: h,
			P: payload,
		}
	}
	return
}

func (me *JsonMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	report.H.Reset()

	// encode payload
	bPayload, err := json.Marshal(report.P)
	if err != nil {
		return
	}

	// encode header
	bHead, err := report.H.Flush(uint64(len(bPayload)))
	if err != nil {
		return
	}

	b = append(bHead, bPayload...)
	return
}

func (me *JsonMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
	var payloads *ReportPayload

	// decode header
	if h == nil {
		h, err = DecodeHeader(b)
		if err != nil {
			return
		}
	}

	// clean registry when leaving
	defer func() {
		if h != nil {
			h.Reset()
		}
	}()

	// decode payload
	err = json.Unmarshal(b[h.Length():], &payloads)
	if err == nil {
		report = &Report{
			H: h,
			P: payloads,
		}
	}

	return
}
