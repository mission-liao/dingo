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
	task, err := ComposeTask("test", []interface{}{int64(1), float64(1.5), "user", "password"})
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
			m: &jsonMarshaller{},
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
			m: &gobMarshaller{},
		},
	})
}
