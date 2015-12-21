package dingo

import (
	"github.com/mission-liao/dingo/transport"
)

func NewOption() *transport.Option {
	return transport.NewOption()
}

var Encode = transport.Encode
var ID = transport.ID
var Status = transport.Status
var ErrCode = transport.ErrCode
