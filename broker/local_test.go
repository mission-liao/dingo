package broker

import (
	"testing"
	"time"

	"github.com/mission-liao/dingo/task"
	"github.com/stretchr/testify/assert"
)

func TestLocalSend(t *testing.T) {
	ass := assert.New(t)
	ivk := task.NewDefaultInvoker()

	for _, v := range []interface{}{
		newLocal(false),
		newLocal(true),
	} {
		sender, receiver := v.(Producer), v.(Consumer)
		rpt := make(chan Receipt, 10)

		// prepare consumer
		tasks, errs, err := receiver.Consume(rpt)
		ass.Nil(err)

		// wait for 1 seconds,
		// make sure mux accept that receipt channel
		<-time.After(1 * time.Second)

		// composing a task
		// note: when converting to/from json, only type of float64
		// would be unchanged.
		tk, err := ivk.ComposeTask("test", "param#1", float64(123))
		ass.NotNil(tk)
		ass.Nil(err)

		// send it
		err = sender.Send(tk)
		ass.Nil(err)

		select {
		case expected, ok := <-tasks:
			if !ok {
				ass.Fail("tasks channel is closed")
			} else {
				ass.NotNil(expected)
				if expected != nil {
					ass.True(expected.Equal(tk))
				}
				rpt <- Receipt{
					Id:     expected.GetId(),
					Status: Status.OK,
				}

			}
		case err, ok := <-errs:
			if !ok {
				ass.Fail("errs channel is closed")
			} else {
				ass.Fail(err.Error())
			}
		}

		// done
		err = receiver.Stop()
		ass.Nil(err)
	}
}

func TestLocalConsumeReceipt(t *testing.T) {
	ass := assert.New(t)
	ivk := task.NewDefaultInvoker()
	rpt := make(chan Receipt, 10)

	var v interface{} = newLocal(false)
	sender, receiver := v.(Producer), v.(Consumer)
	tasks, errs, err := receiver.Consume(rpt)
	ass.Nil(err)

	// wait for 1 seconds,
	// make sure mux accept that receipt channel
	<-time.After(1 * time.Second)

	// compose a task
	tk, err := ivk.ComposeTask("test", "test#1")
	ass.NotNil(tk)
	ass.Nil(err)

	err = sender.Send(tk)
	ass.Nil(err)

	select {
	case expected, ok := <-tasks:
		if !ok {
			ass.Fail("tasks channel is closed")
		} else {
			ass.NotNil(expected)

			// There should be an monitored element
			{
				val := v.(*_local)
				_, ok := val.unhandled[expected.GetId()]
				ass.True(ok)
			}

			rpt <- Receipt{
				Id:     expected.GetId(),
				Status: Status.OK,
			}
		}
	case err, ok := <-errs:
		if !ok {
			ass.Fail("errs channel is closed")
		} else {
			ass.Fail(err.Error())
		}
	}

	// done
	err = receiver.Stop()
	ass.Nil(err)
}
