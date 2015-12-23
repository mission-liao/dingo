package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/assert"
)

type testFakeProducer struct {
	events chan *dingo.Event
}

func (me *testFakeProducer) Expect(int) (err error) { return }
func (me *testFakeProducer) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		me.events,
	}, nil
}
func (me *testFakeProducer) Close() (err error)                             { return }
func (me *testFakeProducer) ProducerHook(id int, p interface{}) (err error) { return }
func (me *testFakeProducer) Send(id dingo.Meta, body []byte) (err error) {
	me.events <- dingo.NewEvent(
		dingo.ObjT.PRODUCER,
		dingo.EventLvl.INFO,
		dingo.EventCode.Generic,
		"Send",
	)
	return
}

type testFakeStore struct {
	events chan *dingo.Event
}

func (me *testFakeStore) Expect(int) (err error) { return }
func (me *testFakeStore) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		me.events,
	}, nil
}
func (me *testFakeStore) Close() (err error) { return }
func (me *testFakeStore) Poll(meta dingo.Meta) (reports <-chan []byte, err error) {
	me.events <- dingo.NewEvent(
		dingo.ObjT.STORE,
		dingo.EventLvl.INFO,
		dingo.EventCode.Generic,
		"Poll",
	)
	return make(chan []byte, 1), nil
}
func (me *testFakeStore) Done(meta dingo.Meta) (err error) { return }

func TestDingoEvent(t *testing.T) {
	// make sure events from backend/broker are published
	ass := assert.New(t)
	app, err := dingo.NewApp("remote", nil)
	ass.Nil(err)
	if err != nil {
		return
	}

	// prepare a caller
	_, _, err = app.Use(&testFakeProducer{make(chan *dingo.Event, 10)}, dingo.ObjT.PRODUCER)
	ass.Nil(err)
	if err != nil {
		return
	}
	_, _, err = app.Use(&testFakeStore{make(chan *dingo.Event, 10)}, dingo.ObjT.STORE)
	ass.Nil(err)
	if err != nil {
		return
	}

	// register a task
	err = app.Register("TestDingoEvent", func() {}, dingo.Encode.Default, dingo.Encode.Default, dingo.ID.Default)
	ass.Nil(err)
	if err != nil {
		return
	}

	// there should be 2 events
	_, events, err := app.Listen(dingo.ObjT.ALL, dingo.EventLvl.INFO, 0)
	ass.Nil(err)
	if err != nil {
		return
	}

	// send a task
	_, err = app.Call("TestDingoEvent", nil)
	ass.Nil(err)
	if err != nil {
		return
	}

	// exactly two event should be received.
	e1 := <-events
	e2 := <-events
	ass.True(e1.Origin|e2.Origin == dingo.ObjT.PRODUCER|dingo.ObjT.STORE)
	ass.True(e1.Level == dingo.EventLvl.INFO)
	ass.True(e2.Level == dingo.EventLvl.INFO)
	ass.True(e1.Payload.(string) == "Send" || e2.Payload.(string) == "Send")
	ass.True(e1.Payload.(string) == "Poll" || e2.Payload.(string) == "Poll")

	// release resource
	ass.Nil(app.Close())
}
