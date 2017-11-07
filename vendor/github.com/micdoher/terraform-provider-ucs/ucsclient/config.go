package ucsclient

type Config struct {
	IpAddress             string
	Username              string
	Password              string
	TslInsecureSkipVerify bool
	LogLevel              int
	LogFilename           string
	AppName               string
}

// Configures and returns a fully initialised UCSClient.
func (c *Config) Client() *UCSClient {
	return NewUCSClient(c)
}
