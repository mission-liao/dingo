package dingo_test

import (
	"testing"

	"github.com/mission-liao/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

//
// Reporter
//

func TestLocalReporter(t *testing.T) {
	ass := assert.New(t)

	var reporter dingo.Reporter
	reporter, err := dingo.NewLocalBackend(dingo.DefaultConfig(), nil)

	// test case for Report/Unbind
	reports := make(chan *dingo.ReportEnvelope, 10)
	_, err = reporter.Report(reports)
	ass.Nil(err)

	// teardown
	reporter.(dingo.Object).Close()
}

//
// Backend generic test cases
//

type localBackendTestSuite struct {
	dingo.BackendTestSuite
}

func TestLocalBackendSuite(t *testing.T) {
	suite.Run(t, &localBackendTestSuite{
		dingo.BackendTestSuite{
			Gen: func() (b dingo.Backend, err error) {
				b, err = dingo.NewLocalBackend(dingo.DefaultConfig(), nil)
				if err == nil {
					err = b.(dingo.Object).Expect(dingo.ObjT.REPORTER | dingo.ObjT.STORE)
				}
				return
			},
		},
	})
}
