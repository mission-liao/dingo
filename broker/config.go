package broker

type Config struct {
	Consumers_ int `json:"Consumers"`
	Local      *_localConfig
	Amqp       *_amqpConfig
	Redis      *_redisConfig
}

func (me *Config) Consumers(count int) *Config {
	me.Consumers_ = count
	return me
}

func Default() *Config {
	return &Config{
		Consumers_: 1,
		Local:      defaultLocalConfig(),
		Amqp:       defaultAmqpConfig(),
		Redis:      defaultRedisConfig(),
	}
}
