package dingo_test

import (
	"encoding/json"
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/suite"
)

//
// local(Broker) + local(Backend), Single App
//

type localSingleAppTestSuite struct {
	dingo.DingoSingleAppTestSuite
	wireBroker  chan []byte
	wireBackend chan *dingo.ReportEnvelope
}

func (me *localSingleAppTestSuite) SetupTest() {
	// reset the 'virtual wire' for every test
	me.wireBroker, me.wireBackend = make(chan []byte, 10), make(chan *dingo.ReportEnvelope, 10)

	me.GenApp = func() (app *dingo.App, err error) {
		app, err = dingo.NewApp("remote", nil)
		if err != nil {
			return
		}

		brk, err := dingo.NewLocalBroker(dingo.DefaultConfig(), me.wireBroker)
		if err != nil {
			return
		}
		_, _, err = app.Use(brk, dingo.ObjT.DEFAULT)
		if err != nil {
			return
		}

		bkd, err := dingo.NewLocalBackend(dingo.DefaultConfig(), me.wireBackend)
		if err != nil {
			return
		}
		_, _, err = app.Use(bkd, dingo.ObjT.DEFAULT)
		if err != nil {
			return
		}

		return
	}

	me.DingoSingleAppTestSuite.SetupTest()
}

func TestDingoLocalSingleSuite(t *testing.T) {
	suite.Run(t, &localSingleAppTestSuite{})
}

//
// test case
//
// those tests are unrelated to which broker/backend used.
// thus we only test it here.
//

func (me *localSingleAppTestSuite) TestIgnoreReport() {
	// initiate workers
	me.Nil(me.App_.Register(
		"TestIgnoreReport", func() {},
		dingo.Encode.Default, dingo.Encode.Default, dingo.ID.Default,
	))
	remain, err := me.App_.Allocate("TestIgnoreReport", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.App_.Call(
		"TestIgnoreReport",
		dingo.NewOption().SetIgnoreReport(true).SetMonitorProgress(true),
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
func (me *testMyMarshaller) EncodeTask(fn interface{}, task *dingo.Task) ([]byte, error) {
	// encode args
	bN, _ := json.Marshal(task.Args()[0])
	bName, _ := json.Marshal(task.Args()[1])
	// encode option
	bOpt, _ := json.Marshal(task.P.O)
	return dingo.ComposeBytes(task.H, [][]byte{bN, bName, bOpt})
}

func (me *testMyMarshaller) DecodeTask(h *dingo.Header, fn interface{}, b []byte) (task *dingo.Task, err error) {
	var (
		n    int
		name string
		o    *dingo.Option
	)

	bs, _ := dingo.DecomposeBytes(h, b)
	json.Unmarshal(bs[0], &n)
	json.Unmarshal(bs[1], &name)
	json.Unmarshal(bs[2], &o)

	task = &dingo.Task{
		H: h,
		P: &dingo.TaskPayload{
			O: o,
			A: []interface{}{n, name},
		},
	}
	return
}

func (me *testMyMarshaller) EncodeReport(fn interface{}, report *dingo.Report) (b []byte, err error) {
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

	return dingo.ComposeBytes(report.H, bs)
}

func (me *testMyMarshaller) DecodeReport(h *dingo.Header, fn interface{}, b []byte) (report *dingo.Report, err error) {
	var (
		msg   string
		count int
		s     int16
		e     *dingo.Error
		o     *dingo.Option
	)
	bs, _ := dingo.DecomposeBytes(h, b)
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
	report = &dingo.Report{
		H: h,
		P: &dingo.ReportPayload{
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

func (me *localSingleAppTestSuite) TestMyMarshaller() {
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
	me.Nil(me.App_.Register("TestMyMarshaller", fn, mid, mid, dingo.ID.Default))
	remain, err := me.App_.Allocate("TestMyMarshaller", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	reports, err := me.App_.Call(
		"TestMyMarshaller",
		dingo.NewOption(),
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

func (me *localSingleAppTestSuite) TestCustomMarshaller() {
	fn := func(n int, name string) (msg string, count int) {
		msg = name + "_'s message"
		count = n + 1
		return
	}

	// register marshaller
	mid := int(102)
	err := me.App_.AddMarshaller(mid, &struct {
		testMyInvoker
		dingo.CustomMarshaller
	}{
		testMyInvoker{},
		dingo.CustomMarshaller{Codec: &testMyCodec{}},
	})
	me.Nil(err)

	// allocate workers
	me.Nil(me.App_.Register("TestCustomMarshaller", fn, mid, mid, dingo.ID.Default))
	remain, err := me.App_.Allocate("TestCustomMarshaller", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.App_.Call(
		"TestCustomMarshaller", dingo.NewOption().SetMonitorProgress(true), 12345, "mission",
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

func (me *localSingleAppTestSuite) TestCustomMarshallerWithMinimalFunc() {
	called := false
	fn := func() {
		called = true
	}

	// register marshaller
	mid := int(103)
	err := me.App_.AddMarshaller(mid, &struct {
		testMyInvoker2
		dingo.CustomMarshaller
	}{
		testMyInvoker2{},
		dingo.CustomMarshaller{Codec: &testMyMinimalCodec{}},
	})
	me.Nil(err)

	// allocate workers
	me.Nil(me.App_.Register("TestCustomMarshallerWithMinimalFunc", fn, mid, mid, dingo.ID.Default))
	remain, err := me.App_.Allocate("TestCustomMarshallerWithMinimalFunc", 1, 1)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.App_.Call(
		"TestCustomMarshallerWithMinimalFunc", dingo.NewOption().SetMonitorProgress(true),
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

//
// local(Broker) + local(Backend), multi App
//

type localMultiAppTestSuite struct {
	dingo.DingoMultiAppTestSuite

	wireBroker  chan []byte
	wireBackend chan *dingo.ReportEnvelope
}

func (me *localMultiAppTestSuite) SetupTest() {
	// reset the 'virtual wire' for every test
	me.wireBroker, me.wireBackend = make(chan []byte, 10), make(chan *dingo.ReportEnvelope, 10)

	me.GenCaller = func() (app *dingo.App, err error) {
		app, err = dingo.NewApp("remote", nil)
		me.Nil(err)
		if err != nil {
			return
		}

		brk, err := dingo.NewLocalBroker(dingo.DefaultConfig(), me.wireBroker)
		me.Nil(err)
		if err != nil {
			return
		}
		_, _, err = app.Use(brk, dingo.ObjT.PRODUCER)
		me.Nil(err)
		if err != nil {
			return
		}

		bkd, err := dingo.NewLocalBackend(dingo.DefaultConfig(), me.wireBackend)
		me.Nil(err)
		if err != nil {
			return
		}
		_, _, err = app.Use(bkd, dingo.ObjT.STORE)
		me.Nil(err)
		if err != nil {
			return
		}

		return
	}

	me.GenWorker = func() (app *dingo.App, err error) {
		app, err = dingo.NewApp("remote", nil)
		me.Nil(err)
		if err != nil {
			return
		}

		brk, err := dingo.NewLocalBroker(dingo.DefaultConfig(), me.wireBroker)
		me.Nil(err)
		if err != nil {
			return
		}
		_, _, err = app.Use(brk, dingo.ObjT.CONSUMER)
		me.Nil(err)
		if err != nil {
			return
		}

		bkd, err := dingo.NewLocalBackend(dingo.DefaultConfig(), me.wireBackend)
		me.Nil(err)
		if err != nil {
			return
		}
		_, _, err = app.Use(bkd, dingo.ObjT.REPORTER)
		me.Nil(err)
		if err != nil {
			return
		}

		return
	}

	me.DingoMultiAppTestSuite.SetupTest()
}

func TestDingoLocalMultiAppSuite(t *testing.T) {
	suite.Run(t, &localMultiAppTestSuite{
		dingo.DingoMultiAppTestSuite{
			CountOfCallers: 1,
			CountOfWorkers: 1,
		},
		nil,
		nil,
	})
}

//
// local mode
//

type localModeTestSuite struct {
	dingo.DingoSingleAppTestSuite
}

func TestDingoLocalModeTestSuite(t *testing.T) {
	suite.Run(t, &localModeTestSuite{
		dingo.DingoSingleAppTestSuite{
			GenApp: func() (*dingo.App, error) {
				return dingo.NewApp("local", nil)
			},
		},
	})
}
