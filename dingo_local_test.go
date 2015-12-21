package dingo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

//
// local(Broker) + local(Backend)
//

type localTestSuite struct {
	DingoTestSuite
	wireBroker  chan []byte
	wireBackend chan *ReportEnvelope
}

func (me *localTestSuite) SetupSuite() {
	me.reset()
	me.DingoTestSuite.SetupSuite()
}

func (me *localTestSuite) SetupTest() {
	me.reset()
}

func (me *localTestSuite) reset() {
	// reset the 'virtual wire' for every test
	me.wireBroker, me.wireBackend = make(chan []byte, 10), make(chan *ReportEnvelope, 10)

	me.DingoTestSuite.GenBroker = func() (v interface{}, err error) {
		v, err = NewLocalBroker(DefaultConfig(), me.wireBroker)
		return
	}
	me.DingoTestSuite.GenBackend = func() (b Backend, err error) {
		b, err = NewLocalBackend(DefaultConfig(), me.wireBackend)
		return
	}
}

func TestDingoLocalSuite(t *testing.T) {
	suite.Run(t, &localTestSuite{})
}

//
// test case
//
// those tests are unrelated to which broker/backend used.
// thus we only test it here.
//

func (me *localTestSuite) TestIgnoreReport() {
	// initiate workers
	me.Nil(me.App_.Register(
		"TestIgnoreReport", func() {},
		Encode.Default, Encode.Default, ID.Default,
	))
	remain, err := me.App_.Allocate("TestIgnoreReport", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.App_.Call(
		"TestIgnoreReport",
		NewOption().SetIgnoreReport(true).SetMonitorProgress(true),
	)
	me.Nil(err)
	me.Nil(reports)
}

//
// a demo for cutomized marshaller/invoker without reflect,
// error-checking are skipped for simplicity.
//

type testMyMarshaller struct{}

func (me *testMyMarshaller) Prepare(string, interface{}) (err error) { return }
func (me *testMyMarshaller) EncodeTask(fn interface{}, task *Task) ([]byte, error) {
	// encode args
	bN, _ := json.Marshal(task.Args()[0])
	bName, _ := json.Marshal(task.Args()[1])
	// encode option
	bOpt, _ := json.Marshal(task.P.O)
	return ComposeBytes(task.H, [][]byte{bN, bName, bOpt})
}

func (me *testMyMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
	var (
		n    int
		name string
		o    *Option
	)

	bs, _ := DecomposeBytes(h, b)
	json.Unmarshal(bs[0], &n)
	json.Unmarshal(bs[1], &name)
	json.Unmarshal(bs[2], &o)

	task = &Task{
		H: h,
		P: &TaskPayload{
			O: o,
			A: []interface{}{n, name},
		},
	}
	return
}

func (me *testMyMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	bs := [][]byte{}

	// encode returns
	if report.OK() {
		bMsg, _ := json.Marshal(report.Return()[0])
		bCount, _ := json.Marshal(report.Return()[1])
		bs = append(bs, bMsg, bCount)
	}

	// encode status
	bStatus, _ := json.Marshal(report.Status())
	// encode error
	bError, _ := json.Marshal(report.Error())
	// encode option
	bOpt, _ := json.Marshal(report.Option())
	bs = append(bs, bStatus, bError, bOpt)

	return ComposeBytes(report.H, bs)
}

func (me *testMyMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
	var (
		msg   string
		count int
		s     int16
		e     *Error
		o     *Option
	)
	bs, _ := DecomposeBytes(h, b)
	if len(bs) > 3 {
		// the report might, or might not containing
		// returns.
		json.Unmarshal(bs[0], &msg)
		json.Unmarshal(bs[1], &count)
		bs = bs[2:]
	}
	json.Unmarshal(bs[0], &s)
	json.Unmarshal(bs[1], &e)
	json.Unmarshal(bs[2], &o)
	report = &Report{
		H: h,
		P: &ReportPayload{
			S: s,
			E: e,
			O: o,
			R: []interface{}{msg, count},
		},
	}
	return
}

type testMyInvoker struct{}

func (me *testMyInvoker) Call(f interface{}, param []interface{}) ([]interface{}, error) {
	msg, count := f.(func(int, string) (string, int))(
		param[0].(int),
		param[1].(string),
	)
	return []interface{}{msg, count}, nil
}

func (me *testMyInvoker) Return(f interface{}, returns []interface{}) ([]interface{}, error) {
	return returns, nil
}

func (me *localTestSuite) TestMyMarshaller() {
	fn := func(n int, name string) (msg string, count int) {
		msg = name + "_'s message"
		count = n + 1
		return
	}

	// register marshaller
	mid := int(101)
	err := me.App_.AddMarshaller(mid, &struct {
		testMyInvoker
		testMyMarshaller
	}{})
	me.Nil(err)

	// allocate workers
	me.Nil(me.App_.Register("TestMyMarshaller", fn, mid, mid, ID.Default))
	remain, err := me.App_.Allocate("TestMyMarshaller", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	reports, err := me.App_.Call(
		"TestMyMarshaller",
		NewOption(),
		12345, "mission",
	)
	me.Nil(err)
	r := <-reports
	me.Equal("mission_'s message", r.Return()[0].(string))
	me.Equal(int(12346), r.Return()[1].(int))
}

type testMyCodec struct{}

func (me *testMyCodec) Prepare(name string, fn interface{}) (err error) { return }
func (me *testMyCodec) EncodeArgument(fn interface{}, val []interface{}) ([][]byte, error) {
	bN, _ := json.Marshal(val[0])
	bName, _ := json.Marshal(val[1])
	return [][]byte{bN, bName}, nil
}
func (me *testMyCodec) EncodeReturn(fn interface{}, val []interface{}) ([][]byte, error) {
	bMsg, _ := json.Marshal(val[0])
	bCount, _ := json.Marshal(val[1])
	return [][]byte{bMsg, bCount}, nil
}
func (me *testMyCodec) DecodeArgument(fn interface{}, bs [][]byte) ([]interface{}, error) {
	var (
		n    int
		name string
	)
	json.Unmarshal(bs[0], &n)
	json.Unmarshal(bs[1], &name)
	return []interface{}{n, name}, nil
}
func (me *testMyCodec) DecodeReturn(fn interface{}, bs [][]byte) ([]interface{}, error) {
	var (
		msg   string
		count int
	)
	json.Unmarshal(bs[0], &msg)
	json.Unmarshal(bs[1], &count)
	return []interface{}{msg, count}, nil
}

func (me *localTestSuite) TestCustomMarshaller() {
	fn := func(n int, name string) (msg string, count int) {
		msg = name + "_'s message"
		count = n + 1
		return
	}

	// register marshaller
	mid := int(102)
	err := me.App_.AddMarshaller(mid, &struct {
		testMyInvoker
		CustomMarshaller
	}{
		testMyInvoker{},
		CustomMarshaller{Codec: &testMyCodec{}},
	})
	me.Nil(err)

	// allocate workers
	me.Nil(me.App_.Register("TestCustomMarshaller", fn, mid, mid, ID.Default))
	remain, err := me.App_.Allocate("TestCustomMarshaller", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.App_.Call(
		"TestCustomMarshaller", NewOption().SetMonitorProgress(true), 12345, "mission",
	)
	me.Nil(err)

finished:
	for {
		select {
		case r := <-reports:
			if r.OK() {
				me.Equal("mission_'s message", r.Return()[0].(string))
				me.Equal(int(12346), r.Return()[1].(int))
			}
			if r.Fail() {
				me.Fail("this task is failed, unexpected")
			}
			if r.Done() {
				break finished
			}
		}
	}
}

type testMyInvoker2 struct{}

func (me *testMyInvoker2) Call(f interface{}, param []interface{}) ([]interface{}, error) {
	f.(func())()
	return nil, nil
}

func (me *testMyInvoker2) Return(f interface{}, returns []interface{}) ([]interface{}, error) {
	return returns, nil
}

type testMyMinimalCodec struct{}

func (me *testMyMinimalCodec) Prepare(name string, fn interface{}) (err error) { return }
func (me *testMyMinimalCodec) EncodeArgument(fn interface{}, val []interface{}) (bs [][]byte, err error) {
	return
}
func (me *testMyMinimalCodec) EncodeReturn(fn interface{}, val []interface{}) (bs [][]byte, err error) {
	return
}
func (me *testMyMinimalCodec) DecodeArgument(fn interface{}, bs [][]byte) (val []interface{}, err error) {
	return
}
func (me *testMyMinimalCodec) DecodeReturn(fn interface{}, bs [][]byte) (val []interface{}, err error) {
	return
}

func (me *localTestSuite) TestCustomMarshallerWithMinimalFunc() {
	called := false
	fn := func() {
		called = true
	}

	// register marshaller
	mid := int(103)
	err := me.App_.AddMarshaller(mid, &struct {
		testMyInvoker2
		CustomMarshaller
	}{
		testMyInvoker2{},
		CustomMarshaller{Codec: &testMyMinimalCodec{}},
	})
	me.Nil(err)

	// allocate workers
	me.Nil(me.App_.Register("TestCustomMarshallerWithMinimalFunc", fn, mid, mid, ID.Default))
	remain, err := me.App_.Allocate("TestCustomMarshallerWithMinimalFunc", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.App_.Call(
		"TestCustomMarshallerWithMinimalFunc", NewOption().SetMonitorProgress(true),
	)
	me.Nil(err)

finished:
	for {
		select {
		case r := <-reports:
			if r.OK() {
				me.Len(r.Return(), 0)
			}
			if r.Fail() {
				me.Fail("this task is failed, unexpected")
			}
			if r.Done() {
				break finished
			}
		}
	}
	me.True(called)
}
