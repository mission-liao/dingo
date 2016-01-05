package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
)

var testWork = func(count int, msg string) (string, int) {
	return msg, count
}

func BenchmarkRaw(b *testing.B) {
	var (
		count = 1
		msg   = "test string"
	)
	for i := 0; i < b.N; i++ {
		msg, count = testWork(count, msg)
	}
}

func BenchmarkLocal(b *testing.B) {
	var (
		err   error
		count = 1
		msg   = "test string"
	)
	defer func() {
		if err != nil {
			b.Fatalf("failed: %v\n", err)
		}
	}()

	app, err := dingo.NewApp("local", nil)
	if err != nil {
		return
	}

	err = app.Register("myWork", testWork)
	if err != nil {
		return
	}

	_, err = app.Allocate("myWork", 10, 3)
	if err != nil {
		return
	}

	// setup for invoker
	err = app.AddMarshaller(101, &struct {
		dingo.GobMarshaller
		testMyInvoker
	}{
		dingo.GobMarshaller{},
		testMyInvoker{},
	})
	if err != nil {
		return
	}
	err = app.SetMarshaller("myWork", 101, 101)
	if err != nil {
		return
	}

	// setup for idmaker
	err = app.AddIDMaker(101, &dingo.SeqIDMaker{})
	if err != nil {
		return
	}
	err = app.SetIDMaker("myWork", 101)
	if err != nil {
		return
	}

	// reset timer for preparation
	b.ResetTimer()

	var result *dingo.Result
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result = dingo.NewResult(app.Call("myWork", nil, count, msg))
			result.Wait(0)
		}
	})
}
