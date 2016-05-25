package aws

// Config has driver specific configs
type Config struct {
	Region                    string
	Retries                   int
	CheckInstanceMaxPoll      int
	CheckInstancePollInterval int
}

func defaultConfig() *Config {
	return &Config{
		Retries:                   5,
		CheckInstanceMaxPoll:      30,
		CheckInstancePollInterval: 10,
	}
}
