package meta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(_report{
		Id:     "test_id",
		Status: 101,
		Err_:   &_error{102, "test error"},
		Ret:    nil,
	})
	ass.Nil(err)

	var r _report
	err = json.Unmarshal(body, &r)
	ass.Nil(err)
	if err == nil {
		ass.Equal(101, r.Status)
		ass.Equal("test_id", r.Id)
		ass.Equal(102, r.Err_.GetCode())
		ass.Equal("test error", r.Err_.GetMsg())
	}
}
