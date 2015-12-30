package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/assert"
)

type testFakeProducer struct {
	events chan *dingo.Event
}

func (pdc *testFakeProducer) Expect(int) (err error) { return }
func (pdc *testFakeProducer) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		pdc.events,
	}, nil
}
func (pdc *testFakeProducer) Close() (err error)                             { return }
func (pdc *testFakeProducer) ProducerHook(id int, p interface{}) (err error) { return }
func (pdc *testFakeProducer) Send(id dingo.Meta, body []byte) (err error) {
	pdc.events <- dingo.NewEvent(
		dingo.ObjT.Producer,
		dingo.EventLvl.Info,
		dingo.EventCode.Generic,
		"Send",
	)
	return
}

type testFakeStore struct {
	events chan *dingo.Event
}

func (st *testFakeStore) Expect(int) (err error) { return }
func (st *testFakeStore) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		st.events,
	}, nil
}
func (st *testFakeStore) Close() (err error)                          { return }
func (st *testFakeStore) StoreHook(id int, p interface{}) (err error) { return }
func (st *testFakeStore) Poll(meta dingo.Meta) (reports <-chan []byte, err error) {
	st.events <- dingo.NewEvent(
		dingo.ObjT.Store,
		dingo.EventLvl.Info,
		dingo.EventCode.Generic,
		"Poll",
	)
	return make(chan []byte, 1), nil
}
func (st *testFakeStore) Done(meta dingo.Meta) (err error) { return }

func TestDingoEventFromBackendAndBroker(t *testing.T) {
	// make sure events from backend/broker are published
	ass := assert.New(t)
	app, err := dingo.NewApp("remote", nil)
	ass.Nil(err)
	if err != nil {
		return
	}

	// prepare a caller
	_, _, err = app.Use(&testFakeProducer{make(chan *dingo.Event, 10)}, dingo.ObjT.Producer)
	ass.Nil(err)
	if err != nil {
		return
	}
	_, _, err = app.Use(&testFakeStore{make(chan *dingo.Event, 10)}, dingo.ObjT.Store)
	ass.Nil(err)
	if err != nil {
		return
	}

	// register a task
	err = app.Register("TestDingoEvent", func() {})
	ass.Nil(err)
	if err != nil {
		return
	}

	// there should be 2 events
	_, events, err := app.Listen(dingo.ObjT.All, dingo.EventLvl.Info, 0)
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
	ass.True(e1.Origin|e2.Origin == dingo.ObjT.Producer|dingo.ObjT.Store)
	ass.True(e1.Level == dingo.EventLvl.Info)
	ass.True(e2.Level == dingo.EventLvl.Info)
	ass.True(e1.Payload.(string) == "Send" || e2.Payload.(string) == "Send")
	ass.True(e1.Payload.(string) == "Poll" || e2.Payload.(string) == "Poll")

	// release resource
	ass.Nil(app.Close())
}

type testFakeObject struct {
	events chan *dingo.Event
}

func (obj *testFakeObject) Expect(int) (err error) { return }
func (obj *testFakeObject) Events() ([]<-chan *dingo.Event, error) {
	return []<-chan *dingo.Event{
		obj.events,
	}, nil
}
func (obj *testFakeObject) Close() (err error) { return }

func TestDingoEventLevel(t *testing.T) {
	var (
		err error
		ass = assert.New(t)
	)
	defer func() {
		ass.Nil(err)
	}()

	// attach a fake object
	app, err := dingo.NewApp("local", nil)
	if err != nil {
		return
	}
	defer func() {
		ass.Nil(app.Close())
	}()

	// allocate channels slice
	chs := make([]<-chan *dingo.Event, 4)
	Debug, Info, Warning, Error := 0, 1, 2, 3
	idDebug, idInfo, idWarning, idError := 0, 1, 2, 3
	recv := func(which, lvl int) {
		e := <-chs[which]
		ass.Equal(lvl, e.Level)
	}

	// new a fake object, whose event channel is accessible
	obj := &testFakeObject{events: make(chan *dingo.Event, 1)}
	_, _, err = app.Use(obj, 0)
	if err != nil {
		return
	}

	// subscribe a Debug channel
	idDebug, chs[Debug], err = app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
	if err != nil {
		return
	}

	// subscribe an Info channel
	idInfo, chs[Info], err = app.Listen(dingo.ObjT.All, dingo.EventLvl.Info, 0)
	if err != nil {
		return
	}

	// subscribe a Warning channel
	idWarning, chs[Warning], err = app.Listen(dingo.ObjT.All, dingo.EventLvl.Warning, 0)
	if err != nil {
		return
	}

	// subscribe an Error channel
	idError, chs[Error], err = app.Listen(dingo.ObjT.All, dingo.EventLvl.Error, 0)
	if err != nil {
		return
	}

	// send an error event
	obj.events <- dingo.NewEvent(dingo.ObjT.Bridge, dingo.EventLvl.Error, 0, "error")
	recv(Error, dingo.EventLvl.Error)
	recv(Warning, dingo.EventLvl.Error)
	recv(Info, dingo.EventLvl.Error)
	recv(Debug, dingo.EventLvl.Error)

	// send a warning event
	obj.events <- dingo.NewEvent(dingo.ObjT.Bridge, dingo.EventLvl.Warning, 0, "warning")
	recv(Warning, dingo.EventLvl.Warning)
	recv(Info, dingo.EventLvl.Warning)
	recv(Debug, dingo.EventLvl.Warning)

	// send an info event
	obj.events <- dingo.NewEvent(dingo.ObjT.Bridge, dingo.EventLvl.Info, 0, "info")
	recv(Info, dingo.EventLvl.Info)
	recv(Debug, dingo.EventLvl.Info)

	// send a debug event
	obj.events <- dingo.NewEvent(dingo.ObjT.Bridge, dingo.EventLvl.Debug, 0, "debug")
	recv(Debug, dingo.EventLvl.Debug)

	// stop debug channel, and send an debug event
	err = app.StopListen(idDebug)
	if err != nil {
		return
	}

	// stop info channel, and send an info event
	err = app.StopListen(idInfo)
	if err != nil {
		return
	}

	// stop warning channel, and send an warning event
	err = app.StopListen(idWarning)
	if err != nil {
		return
	}

	// stop error channel, and send an error event
	err = app.StopListen(idError)
	if err != nil {
		return
	}
}

func TestDingoEventOrigin(t *testing.T) {
	var (
		err error
		ass = assert.New(t)
	)
	defer func() {
		ass.Nil(err)
	}()

	app, err := dingo.NewApp("remote", nil)
	if err != nil {
		return
	}
	defer func() {
		ass.Nil(app.Close())
	}()

	// new a fake object, whose event channel is accessible
	obj := &testFakeObject{events: make(chan *dingo.Event, 1)}
	_, _, err = app.Use(obj, 0)
	if err != nil {
		return
	}

	chk := func(ch <-chan *dingo.Event, origin int) {
		e := <-ch
		ass.Equal(origin, e.Origin)
	}

	// subscribe bridge event
	idBridge, chBridge, err := app.Listen(dingo.ObjT.Bridge, dingo.EventLvl.Debug, 0)
	if err != nil {
		return
	}

	// subscribe all event
	idAll, chAll, err := app.Listen(dingo.ObjT.All, dingo.EventLvl.Debug, 0)
	if err != nil {
		return
	}

	// send a mapper event
	obj.events <- dingo.NewEvent(dingo.ObjT.Mapper, dingo.EventLvl.Error, 0, "mapper")
	chk(chAll, dingo.ObjT.Mapper)

	// send a bridge event
	obj.events <- dingo.NewEvent(dingo.ObjT.Bridge, dingo.EventLvl.Error, 0, "bridge")
	chk(chAll, dingo.ObjT.Bridge)
	chk(chBridge, dingo.ObjT.Bridge)

	// send a mapper | bridge event
	obj.events <- dingo.NewEvent(dingo.ObjT.Bridge|dingo.ObjT.Mapper, dingo.EventLvl.Error, 0, "bridge")
	chk(chAll, dingo.ObjT.Bridge|dingo.ObjT.Mapper)
	chk(chBridge, dingo.ObjT.Bridge|dingo.ObjT.Mapper)

	err = app.StopListen(idBridge)
	if err != nil {
		return
	}

	err = app.StopListen(idAll)
	if err != nil {
		return
	}
}

func TestDingoUse(t *testing.T) {
	var (
		err error
		ass = assert.New(t)
	)
	defer func() {
		ass.Nil(err)
	}()

	app, err := dingo.NewApp("remote", nil)
	if err != nil {
		return
	}
	defer func() {
		ass.Nil(app.Close())
	}()

	// new a fake object, whose event channel is accessible
	obj := &testFakeObject{}
	chk := func(id int, used int, err error) {
		ass.Equal(0, used)
		ass.NotNil(err)
	}

	// attach it with different types
	chk(app.Use(obj, dingo.ObjT.Producer))
	chk(app.Use(obj, dingo.ObjT.Consumer))
	chk(app.Use(obj, dingo.ObjT.NamedConsumer))
	chk(app.Use(obj, dingo.ObjT.Reporter))
	chk(app.Use(obj, dingo.ObjT.Store))

	err = app.Register("TestDingoUse", func() {})
	if err != nil {
		return
	}

	remain, err2 := app.Allocate("TestDingoUse", 10, 1)
	ass.Equal(10, remain)
	ass.NotNil(err2)

	reports, err2 := app.Call("TestDingoUse", nil)
	ass.Nil(reports)
	ass.NotNil(err2)
}
