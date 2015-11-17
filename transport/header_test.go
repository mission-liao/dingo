package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeader(t *testing.T) {
	ass := assert.New(t)

	// basic case
	{
		id := "test string dkjgdlfjgldjfg"
		name := "test"
		mashID := int16(19)

		b := EncodeHeader(id, name, mashID)
		m, err := DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, m.Name())
		ass.Equal(mashID, m.MashID())
		ass.Equal(id, m.ID())
	}

	// zero length id
	{
		id := ""
		name := "test"
		mashID := int16(25)

		b := EncodeHeader(id, name, mashID)
		m, err := DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, m.Name())
		ass.Equal(mashID, m.MashID())
		ass.Equal(id, m.ID())
	}

	// zero length name
	{
		id := "kjfkljkjalksdjfkajdlfkjadklfjakldfjkadjflakjdf"
		name := ""
		mashID := int16(35)

		b := EncodeHeader(id, name, mashID)
		m, err := DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, m.Name())
		ass.Equal(mashID, m.MashID())
		ass.Equal(id, m.ID())
	}

	// zero length name, id
	{
		id := ""
		name := ""
		mashID := int16(2345)

		b := EncodeHeader(id, name, mashID)
		m, err := DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(name, m.Name())
		ass.Equal(mashID, m.MashID())
		ass.Equal(id, m.ID())
	}
}
