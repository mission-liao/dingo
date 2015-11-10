package transport

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

//
// marshallers
//

func TestMarshallers(t *testing.T) {
	ass := assert.New(t)
	msh, err := NewMarshallers()
	ass.Nil(err)
	err = msh.Register("test", Encode.JSON, Encode.Gob)
	ass.Nil(err)
	task, err := NewDefaultInvoker().ComposeTask("test", []interface{}{float64(1)})
	task.I = "ID"
	ass.Nil(err)

	{
		// task encoded by json
		b, err := msh.EncodeTask(task)
		ass.Nil(err)

		// make sure it's json
		ass.Equal(string(b[8:]), "{\"I\":\"ID\",\"N\":\"test\",\"A\":[1]}")

		// decode by json
		t, err := msh.DecodeTask(b)
		ass.Nil(err)
		_ = "breakpoint"
		ass.True(t.Equal(task))
	}

	{
		report, err := task.ComposeReport(Status.Done, []interface{}{"test", int64(2)}, errors.New("test error"))
		ass.Nil(err)

		// report encoded by gob
		b, err := msh.EncodeReport(report)

		// make sure it's gob
		ass.Equal(string(b[8:]), "4\xff\x91\x03\x01\x01\x06Report\x01\xff\x92\x00\x01\x05\x01\x01I\x01\f\x00\x01\x01N\x01\f\x00\x01\x01S\x01\x04\x00\x01\x01E\x01\xff\x94\x00\x01\x01R\x01\xff\x82\x00\x00\x00\x1f\xff\x93\x03\x01\x01\x05Error\x01\xff\x94\x00\x01\x02\x01\x01C\x01\x04\x00\x01\x01M\x01\f\x00\x00\x00\f\xff\x81\x02\x01\x02\xff\x82\x00\x01\x10\x00\x008\xff\x92\x01\x02ID\x01\x04test\x01\x06\x01\x02\ntest error\x00\x01\x02\x06string\f\x06\x00\x04test\x05int64\x04\x02\x00\x04\x00")

		// decode by gob
		r, err := msh.DecodeReport(b)
		ass.Nil(err)
		ass.True(r.Equal(report))
	}
}

//
// Marshaller
//

type MarshallerTestSuite struct {
	suite.Suite

	m   Marshaller
	ivk Invoker
}

func (me *MarshallerTestSuite) TestTask() {
	task, err := me.ivk.ComposeTask("test", []interface{}{float64(1.5), "user", "password"})
	me.NotNil(task)
	me.Nil(err)
	if err != nil {
		return
	}

	{
		// encode
		b, err := me.m.EncodeTask(task)
		me.Nil(err)
		me.NotNil(b)

		// decode
		if err == nil {
			t, err := me.m.DecodeTask(b)
			me.Nil(err)
			me.NotNil(t)
			me.True(t.Equal(task))
		}
	}

	// nil case
	{
		_, err := me.m.EncodeTask(nil)
		me.NotNil(err)
	}
}

func (me *MarshallerTestSuite) TestReport() {
	task, err := me.ivk.ComposeTask("test", []interface{}{int64(1), float64(1.5), "user", "password"})
	me.Nil(err)
	if err != nil {
		return
	}

	{
		report, err := task.ComposeReport(
			Status.Sent,
			[]interface{}{int64(2), float64(2.5), "user", "password"},
			errors.New("test error"),
		)
		me.Nil(err)

		// encode
		b, err := me.m.EncodeReport(report)
		me.Nil(err)
		me.NotNil(b)

		// decode
		if err == nil {
			r, err := me.m.DecodeReport(b)
			me.Nil(err)
			me.NotNil(r)
			me.True(r.Equal(report))
		}
	}

	// nil case
	{
		_, err := me.m.EncodeReport(nil)
		me.NotNil(err)
	}
}

//
// JSON
//

type jsonMarshallerTestSuite struct {
	MarshallerTestSuite
}

func TestJsonMarshallerSuite(t *testing.T) {
	suite.Run(t, &jsonMarshallerTestSuite{
		MarshallerTestSuite{
			m:   &jsonMarshaller{},
			ivk: NewDefaultInvoker(),
		},
	})
}

//
// Gob
//

type gobMarshallerTestSuite struct {
	MarshallerTestSuite
}

func TestGobMarshallerSuite(t *testing.T) {
	suite.Run(t, &gobMarshallerTestSuite{
		MarshallerTestSuite{
			m:   &gobMarshaller{},
			ivk: NewDefaultInvoker(),
		},
	})
}
