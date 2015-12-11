package transport

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMgrMarshallers(t *testing.T) {
	ass := assert.New(t)
	trans := NewMgr()
	nothing := func() {}
	ass.Nil(trans.Register("TestMgrMarshallers", nothing, Encode.JSON, Encode.GOB))
	task, err := ComposeTask("TestMgrMarshallers", nil, []interface{}{float64(1)})
	task.H.I = "a2a2da60-9cba-11e5-b690-0002a5d5c51b"
	ass.Nil(err)

	{
		// task encoded by json
		b, err := trans.EncodeTask(task)
		ass.Nil(err)

		// get header
		h, err := DecodeHeader(b)
		ass.Nil(err)

		// make sure it's json
		ass.Equal("{\"O\":{\"IR\":false,\"OR\":false},\"A\":[1]}", string(b[h.Length():]))

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
		ass.Equal("6\xff\x91\x03\x01\x01\rReportPayload\x01\xff\x92\x00\x01\x04\x01\x01S\x01\x04\x00\x01\x01E\x01\xff\x94\x00\x01\x01O\x01\xff\x96\x00\x01\x01R\x01\xff\x82\x00\x00\x00\x1f\xff\x93\x03\x01\x01\x05Error\x01\xff\x94\x00\x01\x02\x01\x01C\x01\x04\x00\x01\x01M\x01\f\x00\x00\x00\"\xff\x95\x03\x01\x01\x06Option\x01\xff\x96\x00\x01\x02\x01\x02IR\x01\x02\x00\x01\x02OR\x01\x02\x00\x00\x00\f\xff\x81\x02\x01\x02\xff\x82\x00\x01\x10\x00\x000\xff\x92\x01\x06\x01\x02\ntest error\x00\x01\x00\x01\x02\x06string\f\x06\x00\x04test\x05int64\x04\x02\x00\x04\x00", string(b[h.Length():]))
		// decode by gob
		r, err := trans.DecodeReport(b)
		ass.Nil(err)
		ass.True(r.Equal(report))
	}
}

func TestMgrInvokers(t *testing.T) {
	called := int(0)
	fn := func(i int64) int8 {
		called = int(i)
		return int8(called)
	}

	ass := assert.New(t)
	trans := NewMgr()
	ass.Nil(trans.Register("TestMgrInvokers", fn, Encode.JSON, Encode.JSON))

	// compose a task, with wrong type of input
	task, err := ComposeTask("TestMgrInvokers", nil, []interface{}{int32(3)})
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

func TestMgrOption(t *testing.T) {
	ass := assert.New(t)
	trans := NewMgr()

	// name doesn't register
	ass.NotNil(trans.SetOption("TestMgrOption", NewOption()))

	// get won't work
	opt, err := trans.GetOption("TestMgrOption")
	ass.NotNil(err)
	ass.Nil(opt)

	// regist a record
	ass.Nil(trans.Register("TestMgrOption", func() {}, Encode.Default, Encode.Default))

	// ok
	ass.Nil(trans.SetOption("TestMgrOption", NewOption()))

	// ok to get
	opt, err = trans.GetOption("TestMgrOption")
	ass.Nil(err)
	ass.NotNil(opt)

	// nil Option
	ass.NotNil(trans.SetOption("TestMgrOption", nil))
}
