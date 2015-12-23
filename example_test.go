package dingo_test

import (
	"fmt"
	"time"

	"github.com/mission-liao/dingo"
)

func ExampleApp_local() {
	// this example demonstrates a job queue runs in background

	var err error
	defer func() {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}()

	// a App in local mode
	app, err := dingo.NewApp("local", nil)
	if err != nil {
		return
	}

	// register a worker function that
	err = app.Register("murmur", func(msg string, repeat int, interval time.Duration) {
		for ; repeat > 0; repeat-- {
			select {
			case <-time.After(interval):
				fmt.Printf("%v\n", msg)
			}
		}
	})
	if err != nil {
		return
	}

	// allocate 5 workers, sharing 1 report channel
	_, err = app.Allocate("murmur", 5, 1)
	if err != nil {
		return
	}

	results := []*dingo.Result{}
	// invoke 10 tasks
	for i := 0; i < 10; i++ {
		results = append(
			results,
			dingo.NewResult(
				// name, option, parameter#1, parameter#2, parameter#3...
				app.Call("murmur", nil, fmt.Sprintf("this is %d speaking", i), 10, 100*time.Millisecond),
			))
	}

	// wait until those tasks are done
	for _, v := range results {
		err = v.Wait(0)
		if err != nil {
			return
		}
	}
}

func ExampleApp_Use_caller() {
	// this example demostrate a caller based on AMQP, used along with ExampleApp_Use_worker
	var err error
	defer func() {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}()
}

func ExampleApp_Use_worker() {
	// this example demonstrate a worker based on AMQP, used along with ExampleApp_Use_caller
	var err error
	defer func() {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}()
}
