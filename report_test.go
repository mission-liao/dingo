package dingo_test

import (
	"encoding/json"
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/assert"
)

func TestReportMarshal(t *testing.T) {
	ass := assert.New(t)

	body, err := json.Marshal(&dingo.Report{
		H: dingo.NewHeader("test_id", "test_name"),
		P: &dingo.ReportPayload{
			S: 101,
			E: &dingo.Error{102, "test error"},
			O: dingo.NewOption().SetIgnoreReport(true).SetMonitorProgress(true),
			R: nil,
		},
	})
	ass.Nil(err)

	var r dingo.Report
	err = json.Unmarshal(body, &r)
	ass.Nil(err)
	if err == nil {
		ass.Equal(int16(101), r.Status())
		ass.Equal("test_id", r.ID())
		ass.Equal(int32(102), r.Error().Code())
		ass.Equal("test error", r.Error().Msg())
		ass.Equal(true, r.Option().IgnoreReport())
		ass.Equal(true, r.Option().MonitorProgress())
	}
}
