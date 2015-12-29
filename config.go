package dingo

type Config struct {
	Mappers_ int `json:"Mappers"`
}

/*Mappers is to set the count of mappers initiated.
Note: "mapper" is the replacement of "Broker" in local mode.
*/
func (cfg *Config) Mappers(count int) *Config {
	cfg.Mappers_ = count
	return cfg
}

func DefaultConfig() *Config {
	return &Config{
		Mappers_: 3,
	}
}
