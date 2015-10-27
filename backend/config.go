package backend

//
type Config struct {
	_local *_localConfig `json:"Local"`
	_amqp  *_amqpConfig  `json:"AMQP"`
}

func (me *Config) Local_() *_localConfig { return me._local }
func (me *Config) AMQP_() *_amqpConfig   { return me._amqp }

func Default() *Config {
	return &Config{
		_local: defaultLocalConfig(),
		_amqp:  defaultAmqpConfig(),
	}
}
