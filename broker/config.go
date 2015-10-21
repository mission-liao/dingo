package broker

type Config struct {
	_consumers int           `json:"Consumers"`
	_local     *_localConfig `json:"Local"`
}

func (me *Config) Consumers(count int) *Config {
	me._consumers = count
	return me
}

func (me *Config) Local_() *_localConfig { return me._local }

func Default() *Config {
	return &Config{
		_consumers: 1,
		_local:     defaultLocalConfig(),
	}
}
