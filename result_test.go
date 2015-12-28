package dingo

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type resultTestSuite struct {
	suite.Suite
	trans *fnMgr
}

func TestResultSuite(t *testing.T) {
	suite.Run(t, &resultTestSuite{
		trans: newFnMgr(),
	})
}

//
// test case
//

func (ts *resultTestSuite) TestNew() {
	// nil channel
	{
		r := NewResult(nil, nil)
		ts.NotNil(r)
		// no matter how many times, same error
		ts.Equal(ResultError.NoChannel, r.Wait(0))
		ts.Equal(ResultError.NoChannel, r.Wait(0))
		ts.Equal(ResultError.NoChannel, r.Wait(0))
	}

	// some error
	{
		r := NewResult(nil, errors.New("test error"))
		ts.NotNil(r)
		ts.Equal("test error", r.Wait(0).Error())
		ts.Equal("test error", r.Wait(0).Error())
		ts.Equal("test error", r.Wait(0).Error())
		ts.Equal("test error", r.Wait(0).Error())
	}

	// ok
	{
		r := NewResult(make(chan *Report, 1), nil)
		ts.NotNil(r)
		ts.Equal(ResultError.Timeout, r.Wait(100*time.Millisecond))
	}

	// channel and error, original error is returned
	{
		r := NewResult(make(chan *Report, 1), errors.New("test error"))
		ts.NotNil(r)
		ts.Equal("test error", r.Wait(0).Error())
		ts.Equal("test error", r.Wait(0).Error())
		ts.Equal("test error", r.Wait(0).Error())
	}
}

func (ts *resultTestSuite) TestWait() {
	ts.Nil(ts.trans.Register(
		"TestWait", func() {},
	))

	// compose a task
	task, err := ts.trans.ComposeTask("TestWait", nil, nil)
	ts.Nil(err)
	if err != nil {
		return
	}

	chk := func(s int16, v []interface{}, e error, t time.Duration) {
		reports := make(chan *Report, 10)
		r := NewResult(reports, nil)
		ts.NotNil(r)
		if r == nil {
			return
		}
		// wait with timeout
		ts.Equal(ResultError.Timeout, r.Wait(100*time.Millisecond))

		// a report
		report, err := task.composeReport(s, v, e)
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- report
		if t != 0 {
			ts.Equal(ResultError.Timeout, r.Wait(t))
		} else {
			err = r.Wait(0)
			ts.Nil(err)
			if err != nil {
				return
			}
			if report.OK() {
				ts.Equal(v, r.Last.Return())
			} else {
				ts.Equal(e.Error(), r.Last.Error().Msg())
			}
		}
	}

	chk(Status.Sent, nil, nil, 100*time.Millisecond)
	chk(Status.Fail, nil, errors.New("test fail"), 0)
	chk(Status.Success, []interface{}{1, "value"}, nil, 0)
}

func (ts *resultTestSuite) TestChannelClose() {
	reports := make(chan *Report, 10)
	r := NewResult(reports, nil)
	close(reports)
	ts.Equal(ResultError.ChannelClosed, r.Wait(0))
}

func (ts *resultTestSuite) TestHandlerWithWait() {
	ts.Nil(ts.trans.Register(
		"TestHandlerWithWait", func() {},
	))

	// compose a task
	task, err := ts.trans.ComposeTask("TestHandlerWithWait", nil, nil)
	ts.Nil(err)
	if err != nil {
		return
	}

	// success -> OnOK -> Wait
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.composeReport(Status.Success, []interface{}{int(1), "test string"}, nil)
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		called := false
		// OnOK
		res.OnOK(func(n int, s string) {
			ts.Equal(int(1), n)
			ts.Equal("test string", s)
			called = true
		})

		// Wait
		ts.Nil(res.Wait(0))
		ts.True(called)
	}

	// success -> Wait -> OnOK
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.composeReport(Status.Success, []interface{}{int(1), "test string"}, nil)
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		called := false
		// Wait
		ts.Nil(res.Wait(0))

		// OnOK
		res.OnOK(func(n int, s string) {
			ts.Equal(int(1), n)
			ts.Equal("test string", s)
			called = true
		})

		ts.True(called)
	}

	// fail -> OnNOK -> Wait
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.composeReport(Status.Fail, nil, errors.New("test string"))
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// OnNOK
		called := false
		res.OnNOK(func(te *Error, e error) {
			ts.Nil(e)
			ts.Equal("test string", te.Msg())
			called = true
		})

		// Wait
		ts.Nil(res.Wait(0))
		ts.True(called)
	}

	// fail -> Wait -> OnNOK
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)

		rep, err := task.composeReport(Status.Fail, nil, errors.New("test string"))
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// Wait
		ts.Nil(res.Wait(0))

		// OnNOK
		called := false
		res.OnNOK(func(te *Error, e error) {
			ts.Nil(e)
			ts.Equal("test string", te.Msg())
			called = true
		})

		ts.True(called)
	}

	// err -> OnNOK -> Wait
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)
		close(reports)

		// OnNOK
		called := false
		res.OnNOK(func(te *Error, e error) {
			ts.Nil(te)
			ts.Equal(ResultError.ChannelClosed, e)
			called = true
		})

		// Wait
		ts.Equal(ResultError.ChannelClosed, res.Wait(0))
		ts.True(called)
	}

	// err -> Wait -> OnNOK
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)
		close(reports)

		// Wait
		ts.Equal(ResultError.ChannelClosed, res.Wait(0))

		// OnNOK
		called := false
		res.OnNOK(func(te *Error, e error) {
			ts.Nil(te)
			ts.Equal(ResultError.ChannelClosed, e)
			called = true
		})

		ts.True(called)
	}
}

func (ts *resultTestSuite) TestThen() {
	ts.Nil(ts.trans.Register(
		"TestThen", func() {},
	))

	// compose a task
	task, err := ts.trans.ComposeTask("TestThen", nil, nil)
	ts.Nil(err)
	if err != nil {
		return
	}

	var wait sync.WaitGroup

	// success
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)

		// no handlers
		ts.Equal(ResultError.NoHandler, res.Then())

		// success
		rep, err := task.composeReport(Status.Success, []interface{}{int(1), "test string"}, nil)
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// OnOK
		res.OnOK(func(n int, s string) {
			ts.Equal(int(1), n)
			ts.Equal("test string", s)
			wait.Done()
		})

		wait.Add(1)
		ts.Nil(res.Then())
		wait.Wait()
	}

	// fail
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)

		// fail
		rep, err := task.composeReport(Status.Fail, nil, errors.New("test string"))
		ts.Nil(err)
		if err != nil {
			return
		}

		reports <- rep
		// OnNOK
		res.OnNOK(func(te *Error, e error) {
			ts.Nil(e)
			ts.Equal("test string", te.Msg())
			wait.Done()
		})

		wait.Add(1)
		ts.Nil(res.Then())
		wait.Wait()
	}

	// error
	{
		reports := make(chan *Report, 10)
		res := NewResult(reports, nil)
		close(reports)

		// OnNOK
		res.OnNOK(func(te *Error, e error) {
			ts.Equal(ResultError.ChannelClosed, e)
			wait.Done()
		})

		wait.Add(1)
		ts.Nil(res.Then())
		wait.Wait()
	}
}

func (ts *resultTestSuite) TestOnOK() {
	// should be panic when fn is nil
	ts.Panics(func() {
		NewResult(nil, nil).OnOK(nil)
	})

	// should be panic when invoking is failed.
	ts.Nil(ts.trans.Register(
		"TestOnOK", func() {},
	))

	// compose a task
	task, err := ts.trans.ComposeTask("TestOnOK", nil, nil)
	ts.Nil(err)
	if err != nil {
		return
	}

	// compose a success report
	rep, err := task.composeReport(Status.Success, []interface{}{int(1), "test string"}, nil)
	ts.Nil(err)
	if err != nil {
		return
	}

	reports := make(chan *Report, 10)
	res := NewResult(reports, nil)
	reports <- rep
	res.Wait(0)

	// a totally wrong handler
	ts.Panics(func() {
		res.OnOK(func(name string, count int) {})
	})
}

func (ts *resultTestSuite) TestChecker() {
	var (
		err    error
		report *Report
	)
	defer func() {
		ts.Nil(err)
	}()
	err = ts.trans.Register("TestChecker", func() {})
	if err != nil {
		return
	}

	task, err := ts.trans.ComposeTask("TestChecker", nil, nil)
	if err != nil {
		return
	}

	// OK
	{
		reports := make(chan *Report, 10)
		result := NewResult(reports, nil)
		ts.False(result.OK())
		ts.False(result.NOK())

		// a Sent report
		report, err = task.composeReport(Status.Sent, nil, nil)
		if err != nil {
			return
		}
		reports <- report
		err2 := result.Wait(20 * time.Millisecond)
		ts.Equal(ResultError.Timeout, err2)
		ts.False(result.OK())
		ts.False(result.NOK())

		// a Success report
		report, err = task.composeReport(Status.Success, nil, nil)
		if err != nil {
			return
		}
		reports <- report
		err = result.Wait(0)
		if err != nil {
			return
		}
		ts.True(result.OK())
		ts.False(result.NOK())
	}

	// NOK, error
	{
		reports := make(chan *Report, 10)
		result := NewResult(reports, errors.New("test error"))
		ts.False(result.OK())
		ts.True(result.NOK())
	}

	// NOK, Error
	{
		reports := make(chan *Report, 10)
		result := NewResult(reports, nil)
		ts.False(result.OK())
		ts.False(result.NOK())

		// a Sent report
		report, err = task.composeReport(Status.Sent, nil, nil)
		if err != nil {
			return
		}
		reports <- report
		err2 := result.Wait(20 * time.Millisecond)
		ts.Equal(ResultError.Timeout, err2)
		ts.False(result.OK())
		ts.False(result.NOK())

		// a Success report
		report, err = task.composeReport(Status.Fail, nil, NewErr(0, errors.New("test error")))
		if err != nil {
			return
		}
		reports <- report
		err = result.Wait(0)
		if err != nil {
			return
		}
		ts.False(result.OK())
		ts.True(result.NOK())
	}
}
