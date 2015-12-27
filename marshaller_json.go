package dingo

import (
	"encoding/json"
	"errors"
)

/*JsonMarshaller is a marshaller implemented via json encoding.
Note: this Marshaller can only work with GenericInvoker.
*/
type JsonMarshaller struct{}

func (ms *JsonMarshaller) Prepare(string, interface{}) (err error) {
	return
}

func (ms *JsonMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	task.H.Reset()

	// encode payload
	var (
		bHead, bPayload []byte
	)

	if bPayload, err = json.Marshal(task.P); err != nil {
		return
	}

	// encode header
	if bHead, err = task.H.Flush(uint64(len(bPayload))); err != nil {
		return
	}

	b = append(bHead, bPayload...)
	return
}

func (ms *JsonMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
	// decode header
	if h == nil {
		if h, err = DecodeHeader(b); err != nil {
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
	if err = json.Unmarshal(b[h.Length():], &payload); err == nil {
		task = &Task{
			H: h,
			P: payload,
		}
	}
	return
}

func (ms *JsonMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is not acceptable")
		return
	}

	// reset registry
	report.H.Reset()

	var (
		bHead, bPayload []byte
	)
	// encode payload
	if bPayload, err = json.Marshal(report.P); err != nil {
		return
	}

	// encode header
	if bHead, err = report.H.Flush(uint64(len(bPayload))); err != nil {
		return
	}

	b = append(bHead, bPayload...)
	return
}

func (ms *JsonMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
	var payloads *ReportPayload

	// decode header
	if h == nil {
		if h, err = DecodeHeader(b); err != nil {
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
	if err = json.Unmarshal(b[h.Length():], &payloads); err == nil {
		report = &Report{
			H: h,
			P: payloads,
		}
	}

	return
}
