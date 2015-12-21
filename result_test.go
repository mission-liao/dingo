package dingo

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

type resultTestSuite struct {
	suite.Suite
	trans *transport.Mgr
}

func TestResultSuite(t *testing.T) {
	suite.Run(t, &resultTestSuite{
		trans: transport.NewMgr(),
	})
}

//
// test case
//

func (me *resultTestSuite) TestNew() {
	// nil channel
	{
		r := NewResult(nil, nil)
		me.NotNil(r)
		// no matter how many times, same error
		me.Equal(ResultError.NoChannel, r.Wait(0))
		me.Equal(ResultError.NoChannel, r.Wait(0))
		me.Equal(ResultError.NoChannel, r.Wait(0))
	}

	// some error
	{
		r := NewResult(nil, errors.New("test error"))
		me.NotNil(r)
		me.Equal("test error", r.Wait(0).Error())
		me.Equal("test error", r.Wait(0).Error())
		me.Equal("test error", r.Wait(0).Error())
		me.Equal("test error", r.Wait(0).Error())
	}

	// ok
	{
		r := NewResult(make(chan *transport.Report, 1), nil)
		me.NotNil(r)
		me.Equal(ResultError.Timeout, r.Wait(100*time.Millisecond))
	}

	// channel and error, original error is returned
	{
		r := NewResult(make(chan *transport.Report, 1), errors.New("test error"))
		me.NotNil(r)
		me.Equal("test error", r.Wait(0).Error())
		me.Equal("test error", r.Wait(0).Error())
		me.Equal("test error", r.Wait(0).Error())
	}
}

func (me *resultTestSuite) TestWait() {
	me.Nil(me.trans.Register(
		"TestWait", func() {},
		transport.Encode.Default, transport.Encode.Default, transport.ID.Default,
	))

	// compose a task
	task, err := me.trans.ComposeTask("TestWait", nil, nil)
	me.Nil(err)
	if err != nil {
		return
	}

	chk := func(s int16, v []interface{}, e error, t time.Duration) {
		reports := make(chan *transport.Report, 10)
		r := NewResult(reports, nil)
		me.NotNil(r)
		if r == nil {
			return
		}
		// wait with timeout
		me.Equal(ResultError.Timeout, r.Wait(100*time.Millisecond))

		// a report
		report, err := task.ComposeReport(s, v, e)
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- report
		if t != 0 {
			me.Equal(ResultError.Timeout, r.Wait(t))
		} else {
			err = r.Wait(0)
			me.Nil(err)
			if err != nil {
				return
			}
			if report.OK() {
				me.Equal(v, r.Last.Return())
			} else {
				me.Equal(e.Error(), r.Last.Error().Msg())
			}
		}
	}

	chk(transport.Status.Sent, nil, nil, 100*time.Millisecond)
	chk(transport.Status.Fail, nil, errors.New("test fail"), 0)
	chk(transport.Status.Success, []interface{}{1, "value"}, nil, 0)
}

func (me *resultTestSuite) TestChannelClose() {
	reports := make(chan *transport.Report, 10)
	r := NewResult(reports, nil)
	close(reports)
	me.Equal(ResultError.ChannelClosed, r.Wait(0))
}

func (me *resultTestSuite) TestHandlerWithWait() {
	me.Nil(me.trans.Register(
		"TestHandlerWithWait", func() {},
		transport.Encode.Default, transport.Encode.Default, transport.ID.Default,
	))

	// compose a task
	task, err := me.trans.ComposeTask("TestHandlerWithWait", nil, nil)
	me.Nil(err)
	if err != nil {
		return
	}

	// success -> OnOK -> Wait
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.ComposeReport(transport.Status.Success, []interface{}{int(1), "test string"}, nil)
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		called := false
		// OnOK
		res.OnOK(func(n int, s string) {
			me.Equal(int(1), n)
			me.Equal("test string", s)
			called = true
		})

		// Wait
		me.Nil(res.Wait(0))
		me.True(called)
	}

	// success -> Wait -> OnOK
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.ComposeReport(transport.Status.Success, []interface{}{int(1), "test string"}, nil)
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		called := false
		// Wait
		me.Nil(res.Wait(0))

		// OnOK
		res.OnOK(func(n int, s string) {
			me.Equal(int(1), n)
			me.Equal("test string", s)
			called = true
		})

		me.True(called)
	}

	// fail -> OnNOK -> Wait
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.ComposeReport(transport.Status.Fail, nil, errors.New("test string"))
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// OnNOK
		called := false
		res.OnNOK(func(te *transport.Error, e error) {
			me.Nil(e)
			me.Equal("test string", te.Msg())
			called = true
		})

		// Wait
		me.Nil(res.Wait(0))
		me.True(called)
	}

	// fail -> Wait -> OnNOK
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.ComposeReport(transport.Status.Fail, nil, errors.New("test string"))
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// Wait
		me.Nil(res.Wait(0))

		// OnNOK
		called := false
		res.OnNOK(func(te *transport.Error, e error) {
			me.Nil(e)
			me.Equal("test string", te.Msg())
			called = true
		})

		me.True(called)
	}

	// err -> OnNOK -> Wait
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)
		close(reports)

		// OnNOK
		called := false
		res.OnNOK(func(te *transport.Error, e error) {
			me.Nil(te)
			me.Equal(ResultError.ChannelClosed, e)
			called = true
		})

		// Wait
		me.Equal(ResultError.ChannelClosed, res.Wait(0))
		me.True(called)
	}

	// err -> Wait -> OnNOK
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)
		close(reports)

		// Wait
		me.Equal(ResultError.ChannelClosed, res.Wait(0))

		// OnNOK
		called := false
		res.OnNOK(func(te *transport.Error, e error) {
			me.Nil(te)
			me.Equal(ResultError.ChannelClosed, e)
			called = true
		})

		me.True(called)
	}
}

func (me *resultTestSuite) TestThen() {
	me.Nil(me.trans.Register(
		"TestThen", func() {},
		transport.Encode.Default, transport.Encode.Default, transport.ID.Default,
	))

	// compose a task
	task, err := me.trans.ComposeTask("TestThen", nil, nil)
	me.Nil(err)
	if err != nil {
		return
	}

	var wait sync.WaitGroup

	// success
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)

		// no handlers
		me.Equal(ResultError.NoHandler, res.Then())

		// success
		rep, err := task.ComposeReport(transport.Status.Success, []interface{}{int(1), "test string"}, nil)
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// OnOK
		res.OnOK(func(n int, s string) {
			me.Equal(int(1), n)
			me.Equal("test string", s)
			wait.Done()
		})

		wait.Add(1)
		me.Nil(res.Then())
		wait.Wait()
	}

	// fail
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)

		// fail
		rep, err := task.ComposeReport(transport.Status.Fail, nil, errors.New("test string"))
		me.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// OnNOK
		res.OnNOK(func(te *transport.Error, e error) {
			me.Nil(e)
			me.Equal("test string", te.Msg())
			wait.Done()
		})

		wait.Add(1)
		me.Nil(res.Then())
		wait.Wait()
	}

	// error
	{
		reports := make(chan *transport.Report, 10)
		res := NewResult(reports, nil)
		close(reports)

		// OnNOK
		res.OnNOK(func(te *transport.Error, e error) {
			me.Equal(ResultError.ChannelClosed, e)
			wait.Done()
		})

		wait.Add(1)
		me.Nil(res.Then())
		wait.Wait()
	}
}

func (me *resultTestSuite) TestOnOK() {
	// should be panic when fn is nil
	me.Panics(func() {
		NewResult(nil, nil).OnOK(nil)
	})

	// should be panic when invoking is failed.
	me.Nil(me.trans.Register(
		"TestOnOK", func() {},
		transport.Encode.Default, transport.Encode.Default, transport.ID.Default,
	))

	// compose a task
	task, err := me.trans.ComposeTask("TestOnOK", nil, nil)
	me.Nil(err)
	if err != nil {
		return
	}

	// compose a success report
	rep, err := task.ComposeReport(transport.Status.Success, []interface{}{int(1), "test string"}, nil)
	me.Nil(err)
	if err != nil {
		return
	}

	reports := make(chan *transport.Report, 10)
	res := NewResult(reports, nil)
	reports <- rep
	res.Wait(0)

	// a totally wrong handler
	me.Panics(func() {
		res.OnOK(func(name string, count int) {})
	})
}
