package dingo

import (
	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
)

//
// dingo
//

type Config struct {
	Mappers_ int             `json:"Mappers"`
	Broker_  *broker.Config  `json:"Broker"`
	Backend_ *backend.Config `json:"Backend"`
}

func (me *Config) Mappers(count int) *Config {
	me.Mappers_ = count
	return me
}

func (me *Config) Broker() *broker.Config   { return me.Broker_ }
func (me *Config) Backend() *backend.Config { return me.Backend_ }

func Default() *Config {
	return &Config{
		Broker_:  broker.Default(),
		Backend_: backend.Default(),
		Mappers_: 3,
	}
}
