package dgamqp

import (
	"fmt"
)

type AmqpConfig struct {
	Host_     string `json:"Host"`
	Port_     int    `json:"Port"`
	User_     string `json:"User"`
	Password_ string `json:"Password"`
}

func DefaultAmqpConfig() *AmqpConfig {
	return &AmqpConfig{
		Host_:     "localhost",
		Port_:     5672,
		User_:     "guest",
		Password_: "guest",
	}
}

//
// setter
//

func (me *AmqpConfig) Host(host string) *AmqpConfig {
	me.Host_ = host
	return me
}

func (me *AmqpConfig) Port(port int) *AmqpConfig {
	me.Port_ = port
	return me
}

func (me *AmqpConfig) User(user string) *AmqpConfig {
	me.User_ = user
	return me
}

func (me *AmqpConfig) Password(password string) *AmqpConfig {
	me.Password_ = password
	return me
}

//
// getter
//

func (me *AmqpConfig) Connection() string {
	return fmt.Sprintf("amqp://%v:%v@%v:%d/", me.User_, me.Password_, me.Host_, me.Port_)
}
