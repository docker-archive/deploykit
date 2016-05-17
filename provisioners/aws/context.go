package aws

import (
	"github.com/docker/libmachete"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/context"
)

// See http://blog.golang.org/context on Context pattern
// and http://blog.golang.org/context/userip/userip.go for derived contexts

type contextKey int

const (
	regionKey contextKey = iota
	retriesKey
	configKey
)

// Config has driver specific configs
type Config struct {
	Region                    string `json:"region" yaml:"region"`
	Retries                   int    `json:"retries" yaml:"retries"`
	CheckInstanceMaxPoll      int    `json:"check_instance_max_poll" yaml:"check_instance_max_poll"`
	CheckInstancePollInterval int    `json:"check_instance_poll_interval" yaml:"check_instance_poll_interval"`
}

func defaultConfig() *Config {
	return &Config{
		CheckInstanceMaxPoll:      30,
		CheckInstancePollInterval: 10,
	}
}

// BuildContextFromKVPair is the ContextBuilder that allows the provisioner to configure
// itself at runtime using the given static key-value pair loaded by the framework.
func BuildContextFromKVPair(parent context.Context, m libmachete.KVPair) context.Context {
	t := defaultConfig()
	err := mapstructure.Decode(m, t)
	if err == nil {
		return WithConfig(BuildContext(parent, t.Region, t.Retries), t)
	}
	return parent
}

// BuildContext returns a context that's properly configured with the required context data.
func BuildContext(parent context.Context, region string, retries int) context.Context {
	return WithRetries(WithRegion(parent, region), retries)
}

// WithConfig adds the config to the context
func WithConfig(parent context.Context, cfg *Config) context.Context {
	copy := *cfg
	return context.WithValue(parent, configKey, copy)
}

// WithRegion returns a new context given a parent context and the region.
// For valid value of region, see http://docs.aws.amazon.com/general/latest/gr/rande.html
func WithRegion(parent context.Context, region string) context.Context {
	copy := region
	return context.WithValue(parent, regionKey, &copy)
}

// RegionFromContext returns the Azure region from the request context.
func RegionFromContext(ctx context.Context) (*string, bool) {
	v, ok := ctx.Value(regionKey).(*string)
	return v, ok
}

// WithRetries returns a new context given a parent context and the retries.
func WithRetries(parent context.Context, retries int) context.Context {
	return context.WithValue(parent, retriesKey, retries)
}

// RetriesFromContext returns the Azure retries from the request context.
func RetriesFromContext(ctx context.Context) (int, bool) {
	v, ok := ctx.Value(retriesKey).(int)
	return v, ok
}

// ConfigFromContext returns the config
func ConfigFromContext(ctx context.Context) (Config, bool) {
	v, ok := ctx.Value(configKey).(Config)
	return v, ok
}
