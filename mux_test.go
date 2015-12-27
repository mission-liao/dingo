package dingo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

//
// test case
//

func TestMuxDifferentType(t *testing.T) {
	var (
		err error
		ass = assert.New(t)
	)
	defer func() {
		ass.Nil(err)
	}()

	m := newMux()
	remain, err := m.More(3)
	ass.Equal(0, remain)
	defer m.Close()

	// prepare for string channel
	cStr := make(chan string, 1)
	iStr, err := m.Register(cStr, 0)
	if err != nil {
		return
	}
	defer func() {
		_, err2 := m.Unregister(iStr)
		ass.Nil(err2)
	}()
	ass.Equal(0, iStr)

	// prepare for integer channel
	cInt := make(chan int, 1)
	iInt, err := m.Register(cInt, 0)
	if err != nil {
		return
	}
	defer func() {
		_, err2 := m.Unregister(iInt)
		ass.Nil(err2)
	}()
	ass.NotEqual(0, iInt)

	// add a handler
	oStr := make(chan string, 1)
	oInt := make(chan int, 1)
	m.Handle(func(val interface{}, idx int) {
		switch idx {
		case iStr:
			oStr <- val.(string)
		case iInt:
			oInt <- val.(int)
		default:
			ass.Fail(fmt.Sprintf("unknown index: %v", idx))
		}
	})

	// send a string
	cStr <- "test string"
	{
		v, ok := <-oStr
		ass.True(ok)
		if ok {
			ass.Equal("test string", v)
		}
	}

	// send an integer
	cInt <- 55
	{
		v, ok := <-oInt
		ass.True(ok)
		if ok {
			ass.Equal(55, v)
		}
	}

	close(cStr)
	close(cInt)
	close(oStr)
	close(oInt)
}

func TestMuxChannelClose(t *testing.T) {
	var (
		err error
		ass = assert.New(t)
	)
	defer func() {
		ass.Nil(err)
	}()

	m := newMux()
	remain, err := m.More(1)
	ass.Equal(0, remain)
	defer m.Close()

	ch := make(chan string, 2)
	ch <- "test string 1"
	ch <- "test string 2"
	// close before registering
	close(ch)

	id, err := m.Register(ch, 0)

	seq := 0
	m.Handle(func(val interface{}, idx int) {
		ass.Equal(id, idx)
		switch seq {
		case 0:
			ass.Equal("test string 1", val.(string))
		case 1:
			ass.Equal("test string 2", val.(string))
		default:
			ass.Fail(fmt.Sprintf("unknown index: %v", seq))
		}
		seq++
	})
}

func TestMuxOutputClose(t *testing.T) {
	ass := assert.New(t)

	m := newMux()
	remain, err := m.More(3)
	ass.Equal(0, remain)
	ass.Nil(err)

	ch := make(chan int, 1)
	id, err := m.Register(ch, 0)
	ass.Nil(err)

	o := make(chan int, 1)
	m.Handle(func(val interface{}, idx int) {
		ass.Equal(id, idx)
		o <- int(val.(int))
	})

	ch <- 66

	// close now
	m.Close()

	v, ok := <-o
	ass.True(ok)
	if ok {
		ass.Equal(66, v)
	}

	close(ch)
	close(o)
}

func TestMuxIdGeneration(t *testing.T) {
	ass := assert.New(t)
	m := newMux()
	remain, err := m.More(3)
	ass.Equal(0, remain)
	ass.Nil(err)

	ch1 := make(chan int, 1)
	id, err := m.Register(ch1, 1)
	ass.Equal(id, 1)
	ass.Nil(err)

	// should generate a new id
	ch2 := make(chan int, 1)
	id, err = m.Register(ch2, 1)
	ass.True(id != 1)
	ass.Nil(err)
}
