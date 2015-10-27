package backend

//
type Config struct {
	Local      *_localConfig
	Amqp       *_amqpConfig
	Redis      *_redisConfig
	Reporters_ int `json:"Reporters"`
}

func Default() *Config {
	return &Config{
		Local:      defaultLocalConfig(),
		Amqp:       defaultAmqpConfig(),
		Redis:      defaultRedisConfig(),
		Reporters_: 3,
	}
}

func (me *Config) Reporters(count int) *Config {
	me.Reporters_ = count
	return me
}
