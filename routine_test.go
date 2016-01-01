package dingo

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testNodeHandler struct {
	id   int
	buff []int
}

func (t *testNodeHandler) HandleInput(v interface{}) {}
func (t *testNodeHandler) Done()                     {}
func (t *testNodeHandler) HandleLink(v interface{}) bool {
	obj := v.(*struct {
		id, number int
	})
	if obj.id == t.id {
		t.buff = append(t.buff, obj.number)
		return true
	}
	return false
}

func TestChainRoutinesCloseInputChannel(t *testing.T) {
	// make sure that:
	//  - all packets sent through headNode would be received by correct nodes.
	//  - after closing inputs channels, link channel should still be listened.
	var (
		err          error
		ass          = assert.New(t)
		routineCount = 10
		packetCount  = 3
	)
	defer func() {
		ass.Nil(err)
	}()

	rs := newChainRoutines(func(v interface{}) {
		ass.Nil(v)
	}, make(chan *Event, 100))
	hs := make([]*testNodeHandler, 0, routineCount)
	chs := make([]chan int, 0, routineCount)

	// sending packets
	for i := 0; i < routineCount; i++ {
		for j := 0; j < packetCount; j++ {
			rs.Send(&struct {
				id, number int
			}{
				i, j,
			})
		}
	}

	// adding routines
	for i := 0; i < routineCount; i++ {
		ch := make(chan int, 3)
		h := testNodeHandler{i, make([]int, 0, packetCount)}
		err = rs.Add(ch, &h)
		if err != nil {
			return
		}

		chs = append(chs, ch)
		hs = append(hs, &h)
	}

	// sending packets
	for i := 0; i < routineCount; i++ {
		for j := 0; j < packetCount; j++ {
			rs.Send(&struct {
				id, number int
			}{
				i, j + packetCount,
			})
		}
	}

	// close input channels
	for _, v := range chs {
		close(v)
	}

	// sending packet again
	for i := 0; i < routineCount; i++ {
		for j := 0; j < packetCount; j++ {
			rs.Send(&struct {
				id, number int
			}{
				i, j + packetCount*2,
			})
		}
	}

	// close
	<-time.After(100 * time.Millisecond)
	rs.Close()

	// check result
	exp := []int{}
	for i := 0; i < packetCount*3; i++ {
		exp = append(exp, i)
	}
	for _, v := range hs {
		sort.Ints(v.buff)
		ass.Equal(exp, v.buff)
	}
}

func TestChainRoutinesRace(t *testing.T) {
	// make sure that Add() and Close() wouldn't race-condition
	var (
		ass       = assert.New(t)
		iterCount = 100
		wait      sync.WaitGroup
		rs        = newChainRoutines(func(v interface{}) {
			ass.Nil(v)
		}, make(chan *Event, 100))
		start = make(chan int, 1)
	)

	wait.Add(2)
	go func() {
		defer wait.Done()
		for i := 0; i < iterCount/2; i++ {
			rs.Add(make(chan int), &testNodeHandler{i, []int{}})
		}
		start <- 1
		for i := 0; i < iterCount/2; i++ {
			rs.Add(make(chan int), &testNodeHandler{i, []int{}})
		}
	}()

	go func() {
		defer wait.Done()

		<-start
		for i := 0; i < iterCount; i++ {
			rs.Close()
		}
	}()

	wait.Wait()
}
