package transport

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(&Report{
		H: NewHeader("test_id", "test_name"),
		P: &reportPayload{
			S: 101,
			E: &Error{102, "test error"},
			O: NewOption().SetIgnoreReport(true),
			R: nil,
		},
	})
	ass.Nil(err)

	var r Report
	err = json.Unmarshal(body, &r)
	ass.Nil(err)
	if err == nil {
		ass.Equal(int16(101), r.Status())
		ass.Equal("test_id", r.ID())
		ass.Equal(int64(102), r.Error().Code())
		ass.Equal("test error", r.Error().Msg())
		ass.Equal(true, r.Option().IgnoreReport())
	}
}
