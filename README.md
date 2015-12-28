# dingo

[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/mission-liao/dingo) [![Build Status](https://travis-ci.org/mission-liao/dingo.svg)](https://travis-ci.org/mission-liao/dingo) [![Coverage Status](https://coveralls.io/repos/mission-liao/dingo/badge.svg?branch=master&service=github)](https://coveralls.io/github/mission-liao/dingo?branch=master)

I initiated this project after [machinery](https://github.com/RichardKnop/machinery), which is a great library and tends to provide a replacement of [Celery](http://www.celeryproject.org/) in #golang. The reasons to create (yet) another task library are:
- To make sending tasks as easy as possible
- Await and receive reports through channels. (_channel_ is a natural way to represent asynchronous results)
- I want to get familiar with those concepts of #golagn: **interface**, **routine**, **channel**, and a distributed task framework is a good topic for practice, :)

One important concept I learned from [Celery](http://www.celeryproject.org/) and inherited in _Dingo_ is that __Caller__ and __Worker__ could share the same codebase.
> When you send a task message in Celery, that message will not contain any source code, but only the name of the task you want to execute. This works similarly to how host names work on the internet: every worker maintains a mapping of task names to their actual functions, called the task registry.

Below is a quicklink to go through this README:
- [Demo](README.md#quick-demo)
- [Features](README.md#features)
- [Guide]() TBD
- [Configuration]() TBD
- [Troubleshooting](README.md#troubleshooting)

##Quick Demo
Here is a quick demo of this project in local mode as a background job pool:
```go
package main

import (
	"fmt"
	"github.com/mission-liao/dingo"
)

func main() {
	// initiate a local app
	app, err := dingo.NewApp("local", nil)
	if err != nil {
		return
	}
	// register a worker function
	err = app.Register("add", func(a int, b int) int {
		return a + b
	})
	if err != nil {
		return
	}

	// allocate workers for that function: 2 workers, sharing 1 report channel.
	_, err = app.Allocate("add", 2, 1)

	// wrap the report channel with dingo.Result
	result := dingo.NewResult(app.Call("add", dingo.NewOption(), 2, 3))
	err = result.Wait(0)
	if err != nil {
		return
	}
	result.OnOK(func(sum int) {
		fmt.Printf("result is: %v\n", sum)
	})

	// release resource
	err = app.Close()
	if err != nil {
		return
	}
}
```

##Features

###(Almost) ANY Function Can Be Your Dingo
> Invoking Worker Functions with Arbitary Signatures

These functions can be used as __worker functions__ by dingo:
```go
type Person struct {
  ID int
  Name string
}
func NewEmployee(p *Person, age int) (failed bool) { ... } // struct, OK
func GetEmployees(age int) (employees map[string]*Person) { ... } // map of struct, OK
func DeleteEmployees(names []string) (count int) { ... } // slice, OK
func DoNothing () { ... } // OK
```

The most compatible exchange format is []byte, to marshall in/out your parameters to []byte, we rely these builtin encoders:
 - encoding/json
 - encoding/gob

Type info are deduced from the signatures of worker functions you register. With those type info, parameters are unmarshalled from []byte to cloeset type. A __type correction__ procedure would be applied on those parameters before invoking.

Obviously, it's hard (not impossible) to handle all types in #golang, these are unsupported by dingo as far as I know:
 - interface: unmarshalling requires concrete types. (so __error__ can be marshalled, but can't be un-marshalled)
 - chan: haven't tried yet
 - private field in struct: they are ignore by json/gob, but it's still possible to support them by providing customized marshaller and invoker. (please search 'ExampleCustomMarshaller' for details)
 
And yes, return values of worker functions would also be __type corrected__.

###Throwing and Catching with Your Dingo
> Two Way Binding with Worker Functions

Besides sending arguments, return values from worker functions can be accessed. Every time you initiate a task, you will get a report channel.
```go
reports, err := app.Call("yourTask", nil, arg1, arg2 ...)

// synchronous waiting
r := <-reports

// asynchronous waiting
go func () {
  r := <-reports
}()

// access the return values
if r.OK() {
  var ret []interface{} = r.Return()
  ret[0].(int) // by type assertion
}
```
Or using [dingo.Result](https://godoc.org/github.com/mission-liao/dingo#Result)
```go
result := dingo.NewResult(app.Call("yourTask", nil, arg1, arg2 ...))
// synchronous waiting
if err := result.Wait(0); err != nil {
  var ret []interface{} = result.Last.Return()
}
// or polling with timeout
for dingo.ResultError.Timeout == result.Wait(1 * time.Second) {
  fmt.Println("waiting.....")
}
// or by handler
result.OnOK(func(_1 retType1, _2 retType2 ...) {
  // no more assertion is required
})
result.OnNOK(func(e1 *dingo.Error, e2 error) {
  // any possible error goes here
})
// asynchronous waiting
err = result.Then() // only error when you don't set any handler
```

###Dingo @Home, or Anywhere
> A Distributed Task Framework with Local Mode

You would prefer a small, local worker pool at early development stage, and transfer to a distributed one when stepping in production. In dingo, there is nothing much to do for transfering (besides debugging, :( )

You've seen a demo for local mode, and it's easy to make it distributed by attaching corresponding components at caller-side and worker-side. A demo: [caller](https://godoc.org/github.com/mission-liao/dingo#example-App-Use-Caller) and [worker](https://godoc.org/github.com/mission-liao/dingo#ex-App-Use-Worker).

In short, at __Caller__ side, you need to:
 - register worker functions for tasks
 - config __default-option__, __id-maker__, __marshaller__ for tasks if needed.
 - attach __Producer__, __Store__

And at __Worker__ side, you need to:
 - register the same worker function as the one on __Caller__ side for tasks
 - config __marshaller__ for tasks if needed, the marshaller used for __Caller__ and __Worker__ should be sync.
 - attach __Consumer__ (or __NamedConsumer__), __Reporter__
 - allocate worker routines

###Personalize Your Dingo
> Customizable

Many core behaviors can be customized:
 - Generation of ID for new tasks: [IDMaker](https://godoc.org/github.com/mission-liao/dingo#IDMaker)
 - Parameter Marshalling: [Marshaller](https://godoc.org/github.com/mission-liao/dingo#Marshaller)
 - Worker Function Invoking: [Invoker](https://godoc.org/github.com/mission-liao/dingo#Invoker)
 - Task Publishing/Consuming: [Producer](https://godoc.org/github.com/mission-liao/dingo#Producer)/[Consumer](https://godoc.org/github.com/mission-liao/dingo#Consumer)/[NamedConsumer](https://godoc.org/github.com/mission-liao/dingo#NamedConsumer)
 - Report Publishing/Consuming: [Reporter](https://godoc.org/github.com/mission-liao/dingo#Reporter)/[Store](https://godoc.org/github.com/mission-liao/dingo#Store)

##Troubleshooting
Most APIs in this library are executed asynchronously in separated go-routines. Therefore, we can't report a failure by returning an error. Instead, an event channel could be subscribed for failure events.
```go
// subscribe a event channel
_, events, err := app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
// initiate a go routine to log events
go func () {
  for {
    select e, ok := <-events:
      if !ok {
        // dingo is closed
      }
      // log it
      fmt.Printf("%v\n", e)
  }
}()
```

You can also turn on __Progress Reports__ for more detailed execution reports.
```go
reports, err := app.Call("YourTask", dingo.NewOption().SetMonitorProgress(true), arg1, arg2, ...)
<-reports // the task is received by workers
<-reports // the task is sent to worker functions
<-reports // the final report for either success or failure
```

Besides these, I would like to add more utility/mechanism to help troubleshooting, if anyone have any suggestion, please feel free to raise an issue for discussion.
