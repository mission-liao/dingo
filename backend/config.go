package backend

//
type Config struct {
	_local *_localConfig `json:"Local"`
}

func (me *Config) Local_() *_localConfig { return me._local }

func Default() *Config {
	return &Config{
		_local: defaultLocalConfig(),
	}
}
