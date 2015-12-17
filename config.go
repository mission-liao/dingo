package dingo

type Config struct {
	Mappers_ int `json:"Mappers"`
}

func (me *Config) Mappers(count int) *Config {
	me.Mappers_ = count
	return me
}

func DefaultConfig() *Config {
	return &Config{
		Mappers_: 3,
	}
}
