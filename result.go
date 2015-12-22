package dingo

import (
	"errors"
	"time"
)

var ResultError = struct {
	// the report channel returned from 'dingo' is nil
	NoChannel error
	// timeout
	Timeout error
	// the report channel is closed
	ChannelClosed error
	// there is no handler registered, shouldn't call .Then()
	NoHandler error
}{
	errors.New("channel is nil"),
	errors.New("time out"),
	errors.New("channel closed"),
	errors.New("no handler registered"),
}

/*
 Result is a wrapper of chan *dingo.Report returned from dingo.App.Call,
 taking care of the logic to handle asynchronous result from 'dingo'.

 Example usage:
  r := dingo.NewResult(app.Call(...))

  // blocking until done
  err := r.Wait(0)
  if err == nil {
    r.Last // the last Report
  }

  // polling for every 1 second
  for dingo.ResultError.Timeout == r.Wait(1*time.Second) {
    // logging or ...
  }

 When the task is done, you could register a handler function, whose fingerprint
 is identical to the return part of worker functions. For example, if
 the worker function is:
  func ComposeWords(words []string) (count int, composed string)

 Its corresponding 'OnOK' handler is:
  func (count int, composed string) {...}

 When anything goes wrong, you could register a handler function via 'OnNOK', whose fingerprint is
  func (*Error, error)
 Both failure reports or errors generated in 'Result' object would be passed to this handler,
 at least one of them would not be nil.

 You can register handlers before calling 'Wait', or call 'Wait' before registering handlers. The ordering
 doesn't matter. Those handlers would be called exactly once.
*/
type Result struct {
	Last    *Report
	reports <-chan *Report
	err     error
	fn      interface{}
	efn     func(*Error, error)
	ivok    Invoker
}

/*
 Simply wrap this factory function with the calling to dingo.Call.
  NewResult(app.Call("test", ...))
*/
func NewResult(reports <-chan *Report, err error) (r *Result) {
	r = &Result{
		reports: reports,
		err:     err,
	}

	if reports == nil && err == nil {
		r.err = ResultError.NoChannel
	}

	return
}

/*
 Wait forever or for a period of time. Here is the meaning of return:
  - timeout: wait again later
  - other errors: something wrong.
  - nil: done, you can access the result via 'Last' member.
 When anything other than 'timeout' is returned, the result of subsequent 'Wait'
 would remain the same.

 Registered callback would be triggered when possible.
*/
func (me *Result) Wait(timeout time.Duration) (err error) {
	// call cached handlers
	defer func() {
		if me.fn != nil {
			me.OnOK(me.fn)
		}
		if me.efn != nil {
			me.OnNOK(me.efn)
		}
	}()

	// check if finished
	if me.err != nil {
		err = me.err
		return
	}
	if me.Last != nil && me.Last.Done() {
		return
	}

	out := func(r *Report, ok bool) bool {
		if !ok {
			err = ResultError.ChannelClosed
			me.err = err
			return true
		}

		me.Last = r
		return r.Done()
	}

	if timeout == 0 {
	done:
		for {
			// blocking
			select {
			case r, ok := <-me.reports:
				if out(r, ok) {
					break done
				}
			}
		}
	} else {
		after := time.After(timeout)
	notime:
		for {
			// until timeout
			select {
			case r, ok := <-me.reports:
				if out(r, ok) {
					break notime
				}
			case <-after:
				err = ResultError.Timeout
				break notime
			}
		}
	}

	return
}

/*
 Asynchronous version of 'Result.Wait'
*/
func (me *Result) Then() (err error) {
	if me.fn == nil && me.efn == nil {
		err = ResultError.NoHandler
		return
	}

	go func() {
		me.Wait(0)
	}()

	return
}

/*
 Assign Invoker for Result.OnOK
*/
func (me *Result) SetInvoker(ivok Invoker) {
	me.ivok = ivok
}

/*
 Set the handler for the successful case.
*/
func (me *Result) OnOK(fn interface{}) {
	if fn == nil {
		panic("nil return handler in dingo.result.OnOK")
	}

	if me.Last != nil && me.Last.OK() {
		if me.ivok == nil {
			me.ivok = &LazyInvoker{}
		}
		_, err := me.ivok.Call(fn, me.Last.Return())
		// reset cached handler
		me.fn = nil

		if err != nil {
			panic(err)
		}
	} else {
		me.fn = fn
	}

	return
}

/*
 Set the handler for the failure case.
*/
func (me *Result) OnNOK(efn func(*Error, error)) {
	if efn == nil {
		panic("nil error handler in dingo.result.OnNOK")
	}

	if me.Last != nil {
		if me.Last.Fail() {
			efn(me.Last.Error(), me.err)
			// reset cached
			me.efn = nil
		}
	} else if me.err != nil {
		efn(nil, me.err)
		// reset cached
		me.efn = nil
	} else {
		me.efn = efn
	}
}
