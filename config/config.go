package config // gofmt

type Config struct {
	MaxDepth   int32  `toml:"max_depth"`
	MaxResults int    `toml:"max_results"`
	MaxErrors  int    `toml:"max_errors"`
	URL        string `toml:"url"`         // stylecheck
	AppTimeout int    `toml:"app_timeout"` //in seconds
	ReqTimeout int    `toml:"req_timeout"` //in seconds
}

func NewConfig() *Config {
	return &Config{
		MaxDepth:   3,
		MaxResults: 10,
		MaxErrors:  20,
		URL:        "https://telegram.org", // stylecheck
		AppTimeout: 10,
		ReqTimeout: 2,
	}
}
