package backend

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

	v, err := New("local", Default())
	reporter := v.(Reporter)

	// test case for Report/Unbind
	reports := make(chan *Envelope, 10)
	_, err = reporter.Report(reports)
	ass.Nil(err)

	// teardown
	v.(*_local).Close()
}

//
// Backend generic test cases
//

type LocalBackendTestSuite struct {
	BackendTestSuite
}

func (me *LocalBackendTestSuite) SetupSuite() {
	var (
		err error
	)

	cfg := Default()
	me._backend, err = New("local", cfg)
	me.Nil(err)
	me.BackendTestSuite.SetupSuite()
}

func (me *LocalBackendTestSuite) TearDownSuite() {
	me.BackendTestSuite.TearDownSuite()
}

func TestLocalBackendSuite(t *testing.T) {
	suite.Run(t, &LocalBackendTestSuite{})
}
