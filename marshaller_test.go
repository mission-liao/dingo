package dingo

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

//
// Marshaller
//

type MarshallerTestSuite struct {
	suite.Suite

	m Marshaller
}

func (ts *MarshallerTestSuite) TestTask() {
	task, err := composeTask(
		"test", NewOption().SetIgnoreReport(true).SetMonitorProgress(true),
		[]interface{}{float64(1.5), "user", "password"},
	)
	ts.NotNil(task)
	ts.Nil(err)
	if err != nil {
		return
	}

	fn := func(float64, string, string) {}

	{
		// encode
		b, err := ts.m.EncodeTask(fn, task)
		ts.Nil(err)
		ts.NotNil(b)

		// decode
		if err == nil {
			// provide a fake function as a reference of fingerprint
			t, err := ts.m.DecodeTask(nil, fn, b)
			ts.Nil(err)
			ts.NotNil(t)
			if t != nil {
				ts.True(t.almostEqual(task))
				ts.True(t.Option().IgnoreReport())
			}
		}
	}

	// nil case
	{
		_, err := ts.m.EncodeTask(fn, nil)
		ts.NotNil(err)
	}
}

func (ts *MarshallerTestSuite) TestReport() {
	task, err := composeTask("test", NewOption().SetIgnoreReport(true).SetMonitorProgress(true), nil)
	ts.Nil(err)
	if err != nil {
		return
	}
	fn := func() (a float64, b, c string) { return }

	{
		report, err := task.composeReport(
			Status.Sent,
			[]interface{}{float64(2.5), "user", "password"},
			errors.New("test error"),
		)
		ts.Nil(err)

		// encode
		b, err := ts.m.EncodeReport(fn, report)
		ts.Nil(err)
		ts.NotNil(b)

		// decode
		if err == nil {
			// provide a fake function as a reference of fingerprint
			r, err := ts.m.DecodeReport(nil, fn, b)
			ts.Nil(err)
			ts.NotNil(r)
			if r != nil {
				ts.Equal(report, r)
				ts.True(r.Option().IgnoreReport())
			}
		}
	}

	// nil case
	{
		_, err := ts.m.EncodeReport(fn, nil)
		ts.NotNil(err)
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
			m: &CustomMarshaller{Codec: &JSONSafeCodec{}},
		},
	})
}

//
// CustomMarshaller
//

func TestCustomMarshaller(t *testing.T) {
	ass := assert.New(t)
	fn := func(n int) (count int) { return }
	task, err := composeTask("TestCustomMarshaller", nil, []interface{}{1})
	ass.Nil(err)
	if err != nil {
		return
	}

	// nil Codec
	i := &CustomMarshaller{}

	bs, err := i.EncodeTask(fn, task)
	ass.NotNil(err)
	ass.Nil(bs)

	report, err := task.composeReport(Status.Success, []interface{}{1}, nil)
	ass.Nil(err)
	if err != nil {
		return
	}
	bs, err = i.EncodeReport(fn, report)
	ass.NotNil(err)
	ass.Nil(bs)
}
