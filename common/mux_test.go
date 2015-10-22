package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//
// test case
//

func TestMuxDifferentType(t *testing.T) {
	ass := assert.New(t)

	m := NewMux()
	remain, err := m.More(3)
	ass.Equal(0, remain)
	ass.Nil(err)
	defer m.Close()

	// prepare for string channel
	cStr := make(chan string, 1)
	iStr, err := m.Register(cStr)
	ass.Nil(err)
	ass.NotEqual(0, iStr)

	// prepare for integer channel
	cInt := make(chan int, 1)
	iInt, err := m.Register(cInt)
	ass.Nil(err)
	ass.NotEqual(0, iInt)

	o := m.Out()

	// send a string
	cStr <- "test string"
	v, ok := <-o
	if ok {
		s, ok := v.Value.(string)
		ass.True(ok)
		if ok {
			ass.Equal("test string", s)
		}
		ass.Equal(v.Id, iStr)
	}

	// send an integer
	cInt <- 55
	v, ok = <-o
	ass.True(ok)
	if ok {
		i, ok := v.Value.(int)
		ass.True(ok)
		if ok {
			ass.Equal(55, i)
		}
		ass.Equal(v.Id, iInt)
	}

	close(cStr)
	close(cInt)
}

func TestMuxChannelClose(t *testing.T) {
	ass := assert.New(t)

	m := NewMux()
	remain, err := m.More(3)
	ass.Equal(0, remain)
	ass.Nil(err)
	defer m.Close()

	ch := make(chan string, 2)
	ch <- "test string 1"
	ch <- "test string 2"
	close(ch)

	// close before registering
	id, err := m.Register(ch)
	ass.Nil(err)

	o := m.Out()
	v, ok := <-o
	ass.True(ok)
	if ok {
		s, ok := v.Value.(string)
		ass.True(ok)
		if ok {
			ass.Equal("test string 1", s)
		}
		ass.Equal(v.Id, id)
	}

	v, ok = <-o
	ass.True(ok)
	if ok {
		s, ok := v.Value.(string)
		ass.True(ok)
		if ok {
			ass.Equal("test string 2", s)
		}
		ass.Equal(id, v.Id)
	}
}

func TestMuxOutputClose(t *testing.T) {
	ass := assert.New(t)

	m := NewMux()
	remain, err := m.More(3)
	ass.Equal(0, remain)
	ass.Nil(err)

	ch := make(chan int, 1)
	id, err := m.Register(ch)
	ass.Nil(err)

	ch <- 66
	o := m.Out()

	// close now, output channel is closed
	m.Close()

	v, ok := <-o
	ass.True(ok)
	if ok {
		i, ok := v.Value.(int)
		ass.True(ok)
		if ok {
			ass.Equal(66, i)
		}
		ass.Equal(v.Id, id)
	}

	// the second time should be failed
	v, ok = <-o
	ass.False(ok)

	close(ch)
}
