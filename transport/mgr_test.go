package transport

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshallers(t *testing.T) {
	ass := assert.New(t)
	trans := NewMgr()
	nothing := func() {}
	ass.Nil(trans.Register("test", nothing, Encode.JSON, Encode.GOB, Invoke.Generic, Invoke.Generic))
	task, err := ComposeTask("test", []interface{}{float64(1)})
	task.I = "ID"
	ass.Nil(err)

	{
		// task encoded by json
		b, err := trans.EncodeTask(task)
		ass.Nil(err)

		// get header
		h, err := DecodeHeader(b)
		ass.Nil(err)

		// make sure it's json
		ass.Equal(string(b[h.Length():]), "{\"I\":\"ID\",\"N\":\"test\",\"A\":[1]}")

		// decode by json
		t, err := trans.DecodeTask(b)
		ass.Nil(err)
		ass.True(t.Equal(task))
	}

	{
		report, err := task.ComposeReport(Status.Done, []interface{}{"test", int64(2)}, errors.New("test error"))
		ass.Nil(err)

		// report encoded by gob
		b, err := trans.EncodeReport(report)
		ass.Nil(err)

		// get header
		h, err := DecodeHeader(b)
		ass.Nil(err)

		// make sure it's gob
		ass.Equal(string(b[h.Length():]), "4\xff\x91\x03\x01\x01\x06Report\x01\xff\x92\x00\x01\x05\x01\x01I\x01\f\x00\x01\x01N\x01\f\x00\x01\x01S\x01\x04\x00\x01\x01E\x01\xff\x94\x00\x01\x01R\x01\xff\x82\x00\x00\x00\x1f\xff\x93\x03\x01\x01\x05Error\x01\xff\x94\x00\x01\x02\x01\x01C\x01\x04\x00\x01\x01M\x01\f\x00\x00\x00\f\xff\x81\x02\x01\x02\xff\x82\x00\x01\x10\x00\x008\xff\x92\x01\x02ID\x01\x04test\x01\x06\x01\x02\ntest error\x00\x01\x02\x06string\f\x06\x00\x04test\x05int64\x04\x02\x00\x04\x00")

		// decode by gob
		r, err := trans.DecodeReport(b)
		ass.Nil(err)
		ass.True(r.Equal(report))
	}
}

func TestInvokers(t *testing.T) {
	called := int(0)
	fn := func(i int64) int8 {
		called = int(i)
		return int8(called)
	}

	ass := assert.New(t)
	trans := NewMgr()
	ass.Nil(trans.Register("TestInvokers", fn, Encode.JSON, Encode.JSON, Invoke.Generic, Invoke.Generic))

	// compose a task, with wrong type of input
	task, err := ComposeTask("TestInvokers", []interface{}{int32(3)})
	ass.Nil(err)

	// Call it
	ret, err := trans.Call(task)
	ass.Nil(err)
	ass.Equal(3, called)
	ass.Len(ret, 1)
	ass.Equal(int8(3), ret[0].(int8))

	// Compose a Report, with wrong type of output
	report, err := task.ComposeReport(Status.Done, []interface{}{int32(2)}, nil)
	ass.Nil(err)

	// fix Return, the type of return value would become 'int8'
	ass.Nil(trans.Return(report))
	ass.Len(report.Return(), 1)
	ass.Equal(int8(2), report.Return()[0].(int8))
}
