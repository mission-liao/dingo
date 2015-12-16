package dingo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

//
// Reporter
//

func TestLocalReporter(t *testing.T) {
	ass := assert.New(t)

	var reporter Reporter
	reporter, err := NewLocalBackend(Default())

	// test case for Report/Unbind
	reports := make(chan *ReportEnvelope, 10)
	_, err = reporter.Report(reports)
	ass.Nil(err)

	// teardown
	reporter.(*localBackend).Close()
}

//
// Backend generic test cases
//

type localBackendTestSuite struct {
	BackendTestSuite
}

func (me *localBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	me.Bkd, err = NewLocalBackend(cfg)
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func TestLocalBackendSuite(t *testing.T) {
	suite.Run(t, &localBackendTestSuite{
		BackendTestSuite{
			Gen: func() (b Backend, err error) {
				b, err = NewLocalBackend(Default())
				return
			},
		},
	})
}
