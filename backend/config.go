package backend

//
type Config struct {
	Local *_localConfig
	Amqp  *_amqpConfig
	Redis *_redisConfig
}

func Default() *Config {
	return &Config{
		Local: defaultLocalConfig(),
		Amqp:  defaultAmqpConfig(),
		Redis: defaultRedisConfig(),
	}
}
