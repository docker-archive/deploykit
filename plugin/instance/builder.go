package instance

import (
	"github.com/codedellemc/infrakit.rackhd/monorail"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/pflag"
)

type options struct {
	endpoint string
	username string
	password string
}

// Builder is a ProvisionerBuilder that creates a RackHD instance provisioner
type Builder struct {
	options options
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("rackhd", pflag.PanicOnError)
	flags.StringVar(&b.options.endpoint, "endpoint", "http://localhost:9090", "RackHD API Endpoint")
	flags.StringVar(&b.options.username, "username", "admin", "RackHD Username")
	flags.StringVar(&b.options.password, "password", "admin123", "RackHD Password")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin() (instance.Plugin, error) {
	mc := monorail.New(b.options.endpoint)
	return NewInstancePlugin(mc, b.options.username, b.options.password), nil
}
