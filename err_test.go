package dingo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(&Error{0, "test string"})
	ass.Nil(err)

	var e Error
	err = json.Unmarshal(body, &e)
	ass.Nil(err)

	ass.Equal(int32(0), e.Code())
	ass.Equal("test string", e.Msg())
}

func TestErrNil(t *testing.T) {
	var e *Error
	// should not panic
	NewErr(0, e)
}
