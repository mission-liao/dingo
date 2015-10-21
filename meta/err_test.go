package meta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(&_error{0, "test string"})
	ass.Nil(err)

	var e _error
	err = json.Unmarshal(body, &e)
	ass.Nil(err)

	ass.Equal(0, e.Code)
	ass.Equal("test string", e.Msg)
}
