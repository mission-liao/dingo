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

/*Result is a wrapper of chan *dingo.Report returned from dingo.App.Call,
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

/*NewResult simply wrap this factory function with the calling to dingo.Call.
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

/*Wait is used to wait forever or for a period of time. Here is the meaning of return:
 - timeout: wait again later
 - other errors: something wrong.
 - nil: done, you can access the result via 'Last' member.
When anything other than 'timeout' is returned, the result of subsequent 'Wait'
would remain the same.

Registered callback would be triggered when possible.
*/
func (rt *Result) Wait(timeout time.Duration) (err error) {
	// call cached handlers
	defer func() {
		if rt.fn != nil {
			rt.OnOK(rt.fn)
		}
		if rt.efn != nil {
			rt.OnNOK(rt.efn)
		}
	}()

	// check if finished
	if rt.err != nil {
		err = rt.err
		return
	}
	if rt.Last != nil && rt.Last.Done() {
		return
	}

	out := func(r *Report, ok bool) bool {
		if !ok {
			err = ResultError.ChannelClosed
			rt.err = err
			return true
		}

		rt.Last = r
		return r.Done()
	}

	if timeout == 0 {
	done:
		for {
			// blocking
			select {
			case r, ok := <-rt.reports:
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
			case r, ok := <-rt.reports:
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

/*Then is the asynchronous version of 'Result.Wait'
 */
func (rt *Result) Then() (err error) {
	if rt.fn == nil && rt.efn == nil {
		err = ResultError.NoHandler
		return
	}

	go func() {
		rt.Wait(0)
	}()

	return
}

/*SetInvoker could assign Invoker for Result.OnOK
 */
func (rt *Result) SetInvoker(ivok Invoker) {
	rt.ivok = ivok
}

/*OnOK is used to set the handler for the successful case.
 */
func (rt *Result) OnOK(fn interface{}) {
	if fn == nil {
		panic("nil return handler in dingo.result.OnOK")
	}

	if rt.Last != nil && rt.Last.OK() {
		if rt.ivok == nil {
			rt.ivok = &LazyInvoker{}
		}
		_, err := rt.ivok.Call(fn, rt.Last.Return())
		// reset cached handler
		rt.fn = nil

		if err != nil {
			panic(err)
		}
	} else {
		rt.fn = fn
	}

	return
}

/*OnNOK is used to set the handler for the failure case.
 */
func (rt *Result) OnNOK(efn func(*Error, error)) {
	if efn == nil {
		panic("nil error handler in dingo.result.OnNOK")
	}

	if rt.Last != nil {
		if rt.Last.Fail() {
			efn(rt.Last.Error(), rt.err)
			// reset cached
			rt.efn = nil
		}
	} else if rt.err != nil {
		efn(nil, rt.err)
		// reset cached
		rt.efn = nil
	} else {
		rt.efn = efn
	}
}

/*OK is used to check the status is OK or not
 */
func (rt *Result) OK() bool {
	return rt.Last != nil && rt.Last.OK()
}

/*NOK is used to check the status is NOK or not.
note: !NOK != OK
*/
func (rt *Result) NOK() bool {
	return rt.err != nil || (rt.Last != nil && rt.Last.Fail())
}
