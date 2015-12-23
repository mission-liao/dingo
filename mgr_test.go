package dingo

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMgrMarshallers(t *testing.T) {
	ass := assert.New(t)
	trans := newMgr()
	ass.Nil(trans.Register("TestMgrMarshallers", func() {}, Encode.JSON, Encode.GOB))
	task, err := trans.ComposeTask("TestMgrMarshallers", nil, []interface{}{float64(1)})
	task.H.I = "a2a2da60-9cba-11e5-b690-0002a5d5c51b" // fix ID for testing
	ass.Nil(err)

	{
		// task encoded by json
		b, err := trans.EncodeTask(task)
		ass.Nil(err)

		// get header
		h, err := DecodeHeader(b)
		ass.Nil(err)

		// make sure it's json
		ass.Equal("{\"O\":{\"IR\":false,\"MP\":false},\"A\":[1]}", string(b[h.Length():]))

		// decode by json
		t, err := trans.DecodeTask(b)
		ass.Nil(err)
		ass.Equal(task, t)
	}

	{
		report, err := task.composeReport(Status.Success, []interface{}{"test", int64(2)}, errors.New("test error"))
		ass.Nil(err)

		// report encoded by gob
		b, err := trans.EncodeReport(report)
		ass.Nil(err)

		// get header
		h, err := DecodeHeader(b)
		ass.Nil(err)

		// make sure it's gob -- need a better to make sure if its gob encoded stream
		ass.True(strings.Contains(string(b[h.Length():]), "\x00"))
		// decode by gob
		r, err := trans.DecodeReport(b)
		ass.Nil(err)
		ass.Equal(report, r)
	}
}

func TestMgrInvokers(t *testing.T) {
	called := int(0)
	fn := func(i int64) int8 {
		called = int(i)
		return int8(called)
	}

	ass := assert.New(t)
	trans := newMgr()
	ass.Nil(trans.Register("TestMgrInvokers", fn, Encode.JSON, Encode.JSON))

	// compose a task, with wrong type of input
	task, err := trans.ComposeTask("TestMgrInvokers", nil, []interface{}{int32(3)})
	ass.Nil(err)

	// Call it
	ret, err := trans.Call(task)
	ass.Nil(err)
	ass.Equal(3, called)
	ass.Len(ret, 1)
	ass.Equal(int8(3), ret[0].(int8))

	// Compose a Report, with wrong type of output
	report, err := task.composeReport(Status.Success, []interface{}{int32(2)}, nil)
	ass.Nil(err)

	// fix Return, the type of return value would become 'int8'
	ass.Nil(trans.Return(report))
	ass.Len(report.Return(), 1)
	ass.Equal(int8(2), report.Return()[0].(int8))
}

func TestMgrOption(t *testing.T) {
	ass := assert.New(t)
	trans := newMgr()

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

func TestMgrRegister(t *testing.T) {
	ass := assert.New(t)
	trans := newMgr()

	// register a function, with not-existed marshaller id.
	ass.NotNil(trans.Register("TestMgrResgister", func() {}, 100, 100))

	// register a function with default marshaller id
	ass.Nil(trans.Register("TestMgrMarshallers1", func() {}, Encode.Default, Encode.Default))

	// register something already registered
	ass.NotNil(trans.Register("TestMgrMarshallers1", func() {}, Encode.Default, Encode.Default))
}

type testAlwaysOneIDMaker struct{}

func (me *testAlwaysOneIDMaker) NewID() (string, error) { return "1", nil }

type testAlwaysErrorIDMaker struct{}

func (me *testAlwaysErrorIDMaker) NewID() (string, error) { return "", errors.New("test error") }

func TestMgrIDMaker(t *testing.T) {
	ass := assert.New(t)
	trans := newMgr()

	// register a function
	ass.Nil(trans.Register("TestMgrIDMaker", func() {}, Encode.Default, Encode.Default))

	// add IDMaker(s)
	ass.Nil(trans.AddIdMaker(101, &testAlwaysOneIDMaker{}))
	ass.Nil(trans.AddIdMaker(102, &testAlwaysErrorIDMaker{}))

	// always "1"
	ass.Nil(trans.SetIDMaker("TestMgrIDMaker", 101))
	task, err := trans.ComposeTask("TestMgrIDMaker", nil, nil)
	ass.Nil(err)
	ass.Equal("1", task.ID())

	// always error
	ass.Nil(trans.SetIDMaker("TestMgrIDMaker", 102))
	task, err = trans.ComposeTask("TestMgrIDMaker", nil, nil)
	ass.Equal("test error", err.Error())
	ass.Nil(task)
}
