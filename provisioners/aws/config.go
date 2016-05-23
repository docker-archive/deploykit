package aws

// Config has driver specific configs
type Config struct {
	Region                    string `json:"region" yaml:"region"`
	Retries                   int    `json:"retries" yaml:"retries"`
	CheckInstanceMaxPoll      int    `json:"check_instance_max_poll" yaml:"check_instance_max_poll"`
	CheckInstancePollInterval int    `json:"check_instance_poll_interval" yaml:"check_instance_poll_interval"`
}

func defaultConfig() *Config {
	return &Config{
		Retries:                   5,
		CheckInstanceMaxPoll:      30,
		CheckInstancePollInterval: 10,
	}
}
