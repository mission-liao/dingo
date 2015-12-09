package dingo

import (
	"testing"

	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/transport"
	"github.com/stretchr/testify/suite"
)

//
// local(Broker) + local(Backend)
//

type LocalTestSuite struct {
	DingoTestSuite
}

func (me *LocalTestSuite) SetupSuite() {
	me.DingoTestSuite.SetupSuite()

	// broker
	{
		v, err := broker.New("local", me.cfg.Broker())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.PRODUCER|common.InstT.CONSUMER, used)
	}

	// backend
	{
		v, err := backend.New("local", me.cfg.Backend())
		me.Nil(err)
		_, used, err := me.app.Use(v, common.InstT.DEFAULT)
		me.Nil(err)
		me.Equal(common.InstT.REPORTER|common.InstT.STORE, used)
	}

	me.Nil(me.app.Init(*me.cfg))
}

func TestDingoLocalSuite(t *testing.T) {
	suite.Run(t, &LocalTestSuite{})
}

//
// test case
//

func (me *DingoTestSuite) TestIgnoreReport() {
	// this test is unrelated to which broker/backend used.
	// thus we only test it here.

	remain, err := me.app.Register(
		"TestIgnoreReport", func() {}, 1, 1,
		transport.Encode.Default, transport.Encode.Default,
	)
	me.Equal(0, remain)
	me.Nil(err)

	// initiate a task with an option(IgnoreReport == true)
	reports, err := me.app.Call(
		"TestIgnoreReport",
		transport.NewOption().SetIgnoreReport(true),
	)
	me.Nil(err)
	me.Nil(reports)
}
