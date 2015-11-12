package transport

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(&Report{
		I: "test_id",
		S: 101,
		E: &Error{102, "test error"},
		R: nil,
	})
	ass.Nil(err)

	var r Report
	err = json.Unmarshal(body, &r)
	ass.Nil(err)
	if err == nil {
		ass.Equal(101, r.Status())
		ass.Equal("test_id", r.ID())
		ass.Equal(int64(102), r.Err().Code())
		ass.Equal("test error", r.Err().Msg())
	}
}
