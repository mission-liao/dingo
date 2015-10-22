package dingo

import (
	"github.com/mission-liao/dingo/backend"
	"github.com/mission-liao/dingo/broker"
)

//
// dingo
//

type Config struct {
	Mappers_  int             `json:"Mappers"`
	Monitors_ int             `json:"Monitors"`
	Broker_   *broker.Config  `json:"Broker"`
	Backend_  *backend.Config `json:"Backend"`
}

func (me *Config) Mappers(count int) *Config {
	me.Mappers_ = count
	return me
}

func (me *Config) Monitors(count int) *Config {
	me.Monitors_ = count
	return me
}

func (me *Config) Broker() *broker.Config   { return me.Broker_ }
func (me *Config) Backend() *backend.Config { return me.Backend_ }

func Default() *Config {
	return &Config{
		Broker_:   broker.Default(),
		Backend_:  backend.Default(),
		Mappers_:  3,
		Monitors_: 3,
	}
}
