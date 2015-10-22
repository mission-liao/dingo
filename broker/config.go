package broker

type Config struct {
	_consumers int           `json:"Consumers"`
	_local     *_localConfig `json:"Local"`
	_amqp      *_amqpConfig  `json:"AMQP"`
}

func (me *Config) Consumers(count int) *Config {
	me._consumers = count
	return me
}

func (me *Config) Local_() *_localConfig { return me._local }
func (me *Config) AMQP_() *_amqpConfig   { return me._amqp }

func Default() *Config {
	return &Config{
		_consumers: 1,
		_local:     defaultLocalConfig(),
		_amqp:      defaultAmqpConfig(),
	}
}
