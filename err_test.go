package dingo_test

import (
	"encoding/json"
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/assert"
)

func TestErrMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(&dingo.Error{0, "test string"})
	ass.Nil(err)

	var e dingo.Error
	err = json.Unmarshal(body, &e)
	ass.Nil(err)

	ass.Equal(int32(0), e.Code())
	ass.Equal("test string", e.Msg())
}

func TestErrNil(t *testing.T) {
	var e *dingo.Error
	// should not panic
	dingo.NewErr(0, e)
}
