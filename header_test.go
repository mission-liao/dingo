package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/assert"
)

func TestHeader(t *testing.T) {
	ass := assert.New(t)

	// basic case
	{
		id := "041ebfa0-9b6e-11e5-ae12-0002a5d5c51b"
		name := "test"

		b, err := dingo.NewHeader(id, name).Flush(0)
		ass.Nil(err)

		h, err := dingo.DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, h.Name())
		ass.Equal(id, h.ID())
	}

	// zero length name, should be ok
	{
		id := "4c257820-9b6e-11e5-b7d5-0002a5d5c51b"
		name := ""

		b, err := dingo.NewHeader(id, name).Flush(0)
		ass.Nil(err)

		h, err := dingo.DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, h.Name())
		ass.Equal(id, h.ID())
	}

	// wrong version
	{
		id := "7dd224e0-9b6e-11e5-aa62-0002a5d5c51b"
		name := ""

		b, err := dingo.NewHeader(id, name).Flush(0)
		ass.Nil(err)

		b[0] ^= 0xff
		h, err := dingo.DecodeHeader(b)
		ass.NotNil(err)
		ass.Nil(h)
	}

	// length is not enough
	{
		id := "7dd224e0-9b6e-11e5-aa62-0002a5d5c51b"
		name := ""

		b, err := dingo.NewHeader(id, name).Flush(0)
		ass.Nil(err)

		h, err := dingo.DecodeHeader(b[:17])
		ass.NotNil(err)
		ass.Nil(h)
	}

	// payloads
	{
		id := "7dd224e0-9b6e-11e5-aa62-0002a5d5c51b"
		name := "test"

		h := dingo.NewHeader(id, name)

		// append several dummy payloads
		for i := 0; i < 1000; i++ {
			h.Append(10)
		}

		b, err := h.Flush(0)
		ass.Nil(err)

		h, err = dingo.DecodeHeader(b)
		ass.Nil(err)
		ass.NotNil(h)
		ass.Len(h.Registry(), 1000)
		for _, v := range h.Registry() {
			ass.Equal(uint64(10), v)
		}

		// reset
		h.Reset()
		b, err = h.Flush(0)
		ass.Nil(err)

		h, err = dingo.DecodeHeader(b)
		ass.Nil(err)
		ass.NotNil(h)
		ass.Len(h.Registry(), 0)

		// flush with reset
		for i := 0; i < 1000; i++ {
			h.Append(10)
		}
		ass.Len(h.Registry(), 1000)
		_, err = h.Flush(0)
		ass.Nil(err)
		ass.Len(h.Registry(), 0)
	}

	// short id
	{
		id := "0"
		name := "test"

		b, err := dingo.NewHeader(id, name).Flush(0)
		ass.Nil(err)

		h, err := dingo.DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, h.Name())
		ass.Equal(id, h.ID())
	}
}

func TestDecomposeBytes(t *testing.T) {
	ass := assert.New(t)

	// buffer is smaller than header
	{
		id := "0"
		name := "test"

		h := dingo.NewHeader(id, name)
		h.Append(100000)

		bs, err := dingo.DecomposeBytes(h, make([]byte, 5))
		ass.Nil(bs)
		ass.NotNil(err)
	}

	// buffer is smaller than registry
	{
		id := "0"
		name := "test"

		h := dingo.NewHeader(id, name)
		h.Append(100000)
		b, err := h.Flush(0)
		ass.Nil(err)

		h, err = dingo.DecodeHeader(b)
		ass.Nil(err)
		bs, err := dingo.DecomposeBytes(h, b)
		ass.Len(bs, 0)
		ass.NotNil(err)
	}
}
