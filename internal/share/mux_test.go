package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//
// test case
//

func TestDifferentType(t *testing.T) {
	ass := assert.New(t)

	m := &Mux{}
	m.Init()
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

	o, err := m.Out()
	ass.Nil(err)

	// send a string
	cStr <- "test string"
	v := <-o
	_ = "breakpoint"
	s, ok := v.(string)
	ass.True(ok)
	if ok {
		ass.Equal("test string", s)
	}

	// send an integer
	cInt <- 55
	v = <-o
	i, ok := v.(int)
	ass.True(ok)
	if ok {
		ass.Equal(55, i)
	}

	close(cStr)
	close(cInt)
}

func TestChannelClose(t *testing.T) {
	ass := assert.New(t)

	m := &Mux{}
	m.Init()
	defer m.Close()

	ch := make(chan string, 2)
	ch <- "test string 1"
	ch <- "test string 2"
	close(ch)

	// close before registering
	m.Register(ch)

	o, err := m.Out()
	ass.Nil(err)

	v := <-o
	s, ok := v.(string)
	ass.True(ok)
	if ok {
		ass.Equal("test string 1", s)
	}

	v = <-o
	s, ok = v.(string)
	ass.True(ok)
	if ok {
		ass.Equal("test string 2", s)
	}
}
