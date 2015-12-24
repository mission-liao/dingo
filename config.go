package dingo

type Config struct {
	Mappers_ int `json:"Mappers"`
}

func (cfg *Config) Mappers(count int) *Config {
	cfg.Mappers_ = count
	return cfg
}

func DefaultConfig() *Config {
	return &Config{
		Mappers_: 3,
	}
}
