package config

type Config struct {
	MaxDepth   int32 `toml:"max_depth"`
	MaxResults int `toml:"max_results"`
	MaxErrors  int `toml:"max_errors"`
	Url        string `toml:"url"`
	AppTimeout int `toml:"app_timeout"`//in seconds
	ReqTimeout int `toml:"req_timeout"`//in seconds
}

func NewConfig() *Config {
	return &Config{
		MaxDepth:   3,
		MaxResults: 10,
		MaxErrors:  20,
		Url:        "https://telegram.org",
		AppTimeout: 10,
		ReqTimeout: 2,
	}
}