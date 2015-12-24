# dingo

[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/mission-liao/dingo) [![Build Status](https://travis-ci.org/mission-liao/dingo.svg)](https://travis-ci.org/mission-liao/dingo) [![Code Coverage](http://gocover.io/_badge/github.com/mission-liao/dingo)](http://gocover.io/github.com/mission-liao/dingo)

I initiated this project after [machinery](https://github.com/RichardKnop/machinery), which is a great library and tends to provide a replacement of [Celery](http://www.celeryproject.org/) in #golang. The reasons to create (yet) another task system are:
- to make sending tasks as easy as possible
- callers receive reports through a holy channel.
- I want to get familiar with those concepts of #golagn: **interface**, **routine**, **channel**, and a distributed task framework is a good topic for practice, :)

Here is a quick demo of this project in local mode as a background job pools:
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

###Invoking Worker Functions with Arbitary Fingerprints
The most compatible exchange format is []byte, to marshall in/out your parameters to []byte, we rely these builtin encoders:
 - encoding/json
 - encoding/gob

Type info are deduced from the fingerprints of worker functions. With these type info, parameters are unmarshalled from []byte to cloeset type. A __type correction__ procedure would be applied on those parameters before invoking.

Obviously, it's hard (not impossible) to handle all types in #golang, these are unsupported by dingo as far as I know:
 - interface: unmarshalling requires concrete types
 - chan: haven't tried yet
 - private field in struct: they are ignore by json/gob, but it's still possible to support them by providing customized marshaller and invoker. (please search 'ExampleCustomMarshaller' for details)
 
And yes, return values of worker functions would also be __type corrected__.

###A Distributed Task Framework with Local Mode
You would prefer a small, local worker pool at early stage, and transfer to a distributed one when stepping in production. There is nothing much to do for transfering (besides debugging, :( )

You've seen a demo for local mode, and it's easy to make it distributed by attaching corresponding components at caller-side and worker-side. A demo can be checked: [caller](https://godoc.org/github.com/mission-liao/dingo#example-App-Use-Caller) and [worker](https://godoc.org/github.com/mission-liao/dingo#ex-App-Use-Worker).

###Customizable
Many core behaviors can be customized:
 - Generation of ID for new tasks: [IDMaker](https://godoc.org/github.com/mission-liao/dingo#IDMaker)
 - Parameter Marshalling: [Marshaller](https://godoc.org/github.com/mission-liao/dingo#Marshaller)
 - Worker Function Invoking: [Invoker](https://godoc.org/github.com/mission-liao/dingo#Invoker)
 - Task Publishing/Consuming: [Producer](https://godoc.org/github.com/mission-liao/dingo#Producer)/[Consumer](https://godoc.org/github.com/mission-liao/dingo#Consumer)/[NamedConsumer](https://godoc.org/github.com/mission-liao/dingo#NamedConsumer)
 - Report Publishing/Consuming: [Reporter](https://godoc.org/github.com/mission-liao/dingo#Reporter)/[Store](https://godoc.org/github.com/mission-liao/dingo#Store)

##Troubleshooting
Most APIs in this library are executed asynchronously in separated go-routines. Therefore, we can't report a failure by returning an error. Instead, an event channel could be subscribed for listening error events.
```go
// subscribe a event channel
_, events, err := app.Listen(dingo.ObjT.ALL, dingo.EventLvl.DEBUG, 0)
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
Besides this, I would like to add more utility/mechanism to help troubleshooting, if anyone have any suggestion, please feel free to raise an issue for discussion.
