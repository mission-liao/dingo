package dingo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mission-liao/dingo"
	"github.com/mission-liao/dingo/amqp"
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

	// register a worker function murmur
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

	// release resource
	err = app.Close()
	if err != nil {
		return
	}
}

func ExampleApp_Use_caller() {
	/*
		import (
			"fmt"
			"time"

			"github.com/mission-liao/dingo"
			"github.com/mission-liao/dingo/amqp"
		)
	*/
	// this example demostrate a caller based on AMQP, used along with ExampleApp_Use_worker
	// make sure you install a rabbitmq server locally.
	var err error
	defer func() {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}()

	// an App in remote mode
	app, err := dingo.NewApp("remote", nil)
	if err != nil {
		return
	}

	// attach an AMQP producer to publish your tasks
	broker, err := dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
	if err != nil {
		return
	}
	_, _, err = app.Use(broker, dingo.ObjT.PRODUCER)
	if err != nil {
		return
	}

	// attach an AMQP store to receive reports from datastores.
	backend, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
	if err != nil {
		return
	}
	_, _, err = app.Use(backend, dingo.ObjT.STORE)
	if err != nil {
		return
	}

	// register a work function that murmur
	err = app.Register("murmur", func(speech *struct {
		Prologue string
		Script   []string
	}, interval time.Duration) (countOfSentence int) {
		// speak the prologue
		fmt.Printf("%v:\n", speech.Prologue)
		countOfSentence++

		// speak the script
		for _, v := range speech.Script {
			<-time.After(interval)
			fmt.Printf("%v\n", v)
			countOfSentence++
		}

		// return the total sentence we talked
		return
	})
	if err != nil {
		return
	}

	// compose a script to talk
	script := &struct {
		Prologue string
		Script   []string
	}{
		Script: []string{
			"Today, I'm announcing this library.",
			"It should be easy to use, ",
			"and fun to play with.",
			"Merry X'mas.",
		},
	}

	// invoke 20 tasks
	results := []*dingo.Result{}
	for i := 0; i < 20; i++ {
		script.Prologue = fmt.Sprintf("this is %d speaking", i)
		results = append(results,
			dingo.NewResult(
				// name, option, parameter#1, parameter#2 ...
				app.Call("murmur", nil, script, 100*time.Millisecond),
			))
	}

	// wait until those tasks are done
	for _, v := range results {
		err = v.Wait(0)
		if err != nil {
			return
		}

		// result is accessible
		fmt.Printf("one worker spoke %v sentences\n", v.Last.Return()[0].(int))
	}

	// release resource
	err = app.Close()
	if err != nil {
		return
	}
}

func ExampleApp_Use_worker() {
	/*
		import (
			"fmt"
			"os"
			"time"

			"github.com/mission-liao/dingo"
			"github.com/mission-liao/dingo/amqp"
		)
	*/
	// this example demonstrate a worker based on AMQP, used along with ExampleApp_Use_caller
	// make sure you install a rabbitmq server locally.
	var err error
	defer func() {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}()

	// an App in remote mode
	app, err := dingo.NewApp("remote", nil)
	if err != nil {
		return
	}

	// attach an AMQP consumer to receive tasks
	broker, err := dgamqp.NewBroker(dgamqp.DefaultAmqpConfig())
	if err != nil {
		return
	}
	_, _, err = app.Use(broker, dingo.ObjT.NAMED_CONSUMER)
	if err != nil {
		return
	}

	// attach an AMQP reporter to publish reports
	backend, err := dgamqp.NewBackend(dgamqp.DefaultAmqpConfig())
	if err != nil {
		return
	}
	_, _, err = app.Use(backend, dingo.ObjT.REPORTER)
	if err != nil {
		return
	}

	// register a work function that murmur
	err = app.Register("murmur", func(speech *struct {
		Prologue string
		Script   []string
	}, interval time.Duration) (countOfSentence int) {
		// speak the prologue
		fmt.Printf("%v:\n", speech.Prologue)
		countOfSentence++

		// speak the script
		for _, v := range speech.Script {
			<-time.After(interval)
			fmt.Printf("%v\n", v)
			countOfSentence++
		}

		// return the total sentence we talked
		return
	})
	if err != nil {
		return
	}

	// allocate 1 workers, sharing 1 report channel
	_, err = app.Allocate("murmur", 1, 1)
	if err != nil {
		return
	}

	// wait until a key stroke
	var stroke []byte = make([]byte, 100)
	fmt.Println("waiting for tasks...stop waiting by pressing enter")
	os.Stdin.Read(stroke)

	// release resource
	err = app.Close()
	if err != nil {
		return
	}
}

// implement CustomMarshallerCodec for this worker function
//   func concate(words []string) (ret string)

type testCustomMarshallerCodec struct{}

func (me *testCustomMarshallerCodec) Prepare(name string, fn interface{}) (err error) { return }
func (me *testCustomMarshallerCodec) EncodeArgument(_ interface{}, val []interface{}) (bs [][]byte, err error) {
	fmt.Println("encode argument is called")
	b, err := json.Marshal(val[0])
	if err != nil {
		return
	}
	bs = [][]byte{b}
	return
}
func (me *testCustomMarshallerCodec) DecodeArgument(_ interface{}, bs [][]byte) (val []interface{}, err error) {
	fmt.Println("decode argument is called")
	var input []string
	err = json.Unmarshal(bs[0], &input)
	if err != nil {
		return
	}
	val = []interface{}{input}
	return
}
func (me *testCustomMarshallerCodec) EncodeReturn(_ interface{}, val []interface{}) (bs [][]byte, err error) {
	fmt.Println("encode return is called")
	b, err := json.Marshal(val[0])
	if err != nil {
		return
	}
	bs = [][]byte{b}
	return
}
func (me *testCustomMarshallerCodec) DecodeReturn(_ interface{}, bs [][]byte) (val []interface{}, err error) {
	fmt.Println("decode return is called")
	var ret string
	err = json.Unmarshal(bs[0], &ret)
	if err != nil {
		return
	}
	val = []interface{}{ret}
	return
}

type testMyInvoker3 struct{}

func (me *testMyInvoker3) Call(f interface{}, param []interface{}) ([]interface{}, error) {
	fmt.Println("my invoker is called for Call")
	// type assertion
	ret := f.(func([]string) string)(param[0].([]string))
	return []interface{}{ret}, nil
}
func (me *testMyInvoker3) Return(f interface{}, returns []interface{}) ([]interface{}, error) {
	fmt.Println("my invoker is called for Return")
	return returns, nil
}

func ExampleCustomMarshaller() {
	/*
		import (
			"encoding/json"
			"fmt"

			"github.com/mission-liao/dingo"
		)
	*/
	// this example demonstrate the usage of using a
	// customized marshaller by encoding every parameter
	// in JSON.
	// And invoke it with customized invoker
	var err error
	defer func() {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}()

	// an App in remote mode, with local backend/broker
	app, err := dingo.NewApp("remote", nil)
	if err != nil {
		return
	}

	// attach a local broker.
	broker, err := dingo.NewLocalBroker(dingo.DefaultConfig(), nil)
	if err != nil {
		return
	}
	_, _, err = app.Use(broker, dingo.ObjT.DEFAULT)
	// attach a local backend
	backend, err := dingo.NewLocalBackend(dingo.DefaultConfig(), nil)
	if err != nil {
		return
	}
	_, _, err = app.Use(backend, dingo.ObjT.DEFAULT)
	if err != nil {
		return
	}

	// register customize marshaller & invoker
	err = app.AddMarshaller(101, &struct {
		testMyInvoker3
		dingo.CustomMarshaller
	}{
		testMyInvoker3{},
		dingo.CustomMarshaller{Codec: &testCustomMarshallerCodec{}},
	})
	if err != nil {
		return
	}

	// register worker function
	err = app.Register("concat", func(words []string) (ret string) {
		for _, v := range words {
			ret = ret + v
		}
		return
	})
	if err != nil {
		return
	}

	// change marshaller of worker function
	err = app.SetMarshaller("concat", 101, 101)
	if err != nil {
		return
	}

	// allocate workers
	_, err = app.Allocate("concat", 1, 1)
	if err != nil {
		return
	}

	// trigger the fire...
	result := dingo.NewResult(app.Call("concat", nil, []string{"Merry ", "X", "'mas"}))
	err = result.Wait(0)
	if err != nil {
		return
	}
	result.OnOK(func(ret string) {
		fmt.Printf("%v\n", ret)
	})

	err = app.Close()
	if err != nil {
		return
	}
}
