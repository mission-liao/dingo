package dingo

import (
	"encoding/json"
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

//
// local(Broker) + local(Backend)
//

// TODO: make this private
type LocalTestSuite struct {
	DingoTestSuite
}

func (me *LocalTestSuite) SetupSuite() {
	me.DingoTestSuite.SetupSuite()

	// broker
	{
		v, err := broker.New("local", me.cfg.Broker())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	{
		v, err := backend.New("local", me.cfg.Backend())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)
	}

	me.Nil(me.app.Init(*me.cfg))
}

func TestDingoLocalSuite(t *testing.T) {
	suite.Run(t, &LocalTestSuite{})
}

//
// test case
//
// those tests are unrelated to which broker/backend used.
// thus we only test it here.
//

func (me *LocalTestSuite) TestIgnoreReport() {
	remain, err := me.app.Register(
		"TestIgnoreReport", func() {}, 1, 1,
		transport.Encode.Default, transport.Encode.Default,
	)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.app.Call(
		"TestIgnoreReport",
		transport.NewOption().SetIgnoreReport(true),
	)
	me.Nil(err)
	me.Nil(reports)
}

//
// a demo for cutomized marshaller/invoker without reflect,
// error-checking are skipped for simplicity.
//

type customMarshaller struct{}

func (me *customMarshaller) Prepare(string, interface{}) (err error) { return }
func (me *customMarshaller) EncodeTask(fn interface{}, task *transport.Task) ([]byte, error) {
	// encode args
	bN, _ := json.Marshal(task.Args()[0])
	bName, _ := json.Marshal(task.Args()[1])
	// encode option
	bOpt, _ := json.Marshal(task.P.O)
	return transport.ComposeBytes(task.H, [][]byte{bN, bName, bOpt})
}

func (me *customMarshaller) DecodeTask(h *transport.Header, fn interface{}, b []byte) (task *transport.Task, err error) {
	var (
		n    int
		name string
		o    *transport.Option
	)

	bs, _ := transport.DecomposeBytes(h, b)
	json.Unmarshal(bs[0], &n)
	json.Unmarshal(bs[1], &name)
	json.Unmarshal(bs[2], &o)

	task = &transport.Task{
		H: h,
		P: &transport.TaskPayload{
			O: o,
			A: []interface{}{n, name},
		},
	}
	return
}

func (me *customMarshaller) EncodeReport(fn interface{}, report *transport.Report) (b []byte, err error) {
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

	return transport.ComposeBytes(report.H, bs)
}

func (me *customMarshaller) DecodeReport(h *transport.Header, fn interface{}, b []byte) (report *transport.Report, err error) {
	var (
		msg   string
		count int
		s     int16
		e     *transport.Error
		o     *transport.Option
	)
	bs, _ := transport.DecomposeBytes(h, b)
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
	report = &transport.Report{
		H: h,
		P: &transport.ReportPayload{
			S: s,
			E: e,
			O: o,
			R: []interface{}{msg, count},
		},
	}
	return
}

type customInvoker struct{}

func (me *customInvoker) Call(f interface{}, param []interface{}) ([]interface{}, error) {
	msg, count := f.(func(int, string) (string, int))(
		param[0].(int),
		param[1].(string),
	)
	return []interface{}{msg, count}, nil
}

func (me *customInvoker) Return(f interface{}, returns []interface{}) ([]interface{}, error) {
	return returns, nil
}

func (me *LocalTestSuite) TestCustomMarshaller() {
	fn := func(n int, name string) (msg string, count int) {
		msg = name + "_'s message"
		count = n + 1
		return
	}

	// register marshaller
	mid := int16(101)
	err := me.app.AddMarshaller(mid, &struct {
		customInvoker
		customMarshaller
	}{})
	me.Nil(err)

	// allocate workers
	remain, err := me.app.Register("TestCustomMarshaller", fn, 1, 1, mid, mid)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.app.Call(
		"TestCustomMarshaller", transport.NewOption().SetOnlyResult(true), 12345, "mission",
	)
	me.Nil(err)
	r := <-reports
	me.Equal("mission_'s message", r.Return()[0].(string))
	me.Equal(int(12346), r.Return()[1].(int))
}
