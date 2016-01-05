package dingo

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeqIDMaker(t *testing.T) {
	ass := assert.New(t)

	// test overflow
	{
		m := &SeqIDMaker{i: ^uint64(0)>>1 - 1}
		id, err := m.NewID()
		ass.Nil(err)
		ass.NotEqual("", id)

		id, err = m.NewID()
		ass.NotNil(err)
		ass.Equal("", id)
	}

	// test normal usaage
	{
		m := &SeqIDMaker{}
		id1, err := m.NewID()
		ass.Nil(err)

		id2, err := m.NewID()
		ass.Nil(err)

		num1, err := strconv.Atoi(id1)
		ass.Nil(err)
		num2, err := strconv.Atoi(id2)
		ass.Nil(err)

		ass.Equal(1, num2-num1)
	}
}
