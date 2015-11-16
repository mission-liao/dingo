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
		mashID := int16(19)

		b := EncodeHeader(id, mashID)
		m, err := DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(mashID, m.MashID())
		ass.Equal(id, m.ID())
	}

	// zero length id
	{
		id := ""
		mashID := int16(25)

		b := EncodeHeader(id, mashID)
		m, err := DecodeHeader(b)
		ass.Nil(err)
		ass.Equal(mashID, m.MashID())
		ass.Equal(id, m.ID())
	}
}
