package transport

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

//
// Marshaller
//

type MarshallerTestSuite struct {
	suite.Suite

	m Marshaller
}

func (me *MarshallerTestSuite) TestTask() {
	task, err := ComposeTask("test", []interface{}{float64(1.5), "user", "password"})
	me.NotNil(task)
	me.Nil(err)
	if err != nil {
		return
	}

	fn := func(float64, string, string) {}

	{
		// encode
		b, err := me.m.EncodeTask(fn, task)
		me.Nil(err)
		me.NotNil(b)

		// decode
		if err == nil {
			// provide a fake function as a reference of fingerprint
			t, err := me.m.DecodeTask(nil, fn, b)
			me.Nil(err)
			me.NotNil(t)
			if t != nil {
				me.True(t.Equal(task))
			}
		}
	}

	// nil case
	{
		_, err := me.m.EncodeTask(fn, nil)
		me.NotNil(err)
	}
}

func (me *MarshallerTestSuite) TestReport() {
	task, err := ComposeTask("test", []interface{}{int64(1), float64(1.5), "user", "password"})
	me.Nil(err)
	if err != nil {
		return
	}
	fn := func() (a int64, b float64, c, d string) { return }

	{
		report, err := task.ComposeReport(
			Status.Sent,
			[]interface{}{int64(2), float64(2.5), "user", "password"},
			errors.New("test error"),
		)
		me.Nil(err)

		// encode
		b, err := me.m.EncodeReport(fn, report)
		me.Nil(err)
		me.NotNil(b)

		// decode
		if err == nil {
			_ = "breakpoint"
			// provide a fake function as a reference of fingerprint
			r, err := me.m.DecodeReport(nil, fn, b)
			me.Nil(err)
			me.NotNil(r)
			if r != nil {
				me.True(r.Equal(report))
			}
		}
	}

	// nil case
	{
		_, err := me.m.EncodeReport(fn, nil)
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
			m: &JsonMarshaller{},
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
			m: &GobMarshaller{},
		},
	})
}

//
// JsonSafe
//

type jsonSafeMarshallerTestSuite struct {
	MarshallerTestSuite
}

func TestJsonSafeMarshallerSuite(t *testing.T) {
	suite.Run(t, &jsonSafeMarshallerTestSuite{
		MarshallerTestSuite{
			m: &JsonSafeMarshaller{},
		},
	})
}
