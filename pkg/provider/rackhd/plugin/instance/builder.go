package instance

import (
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/pflag"
	"github.com/spiegela/gorackhd/monorail"
)

// Options contain parameters required to connect to RackHD
type Options struct {
	Endpoint string
	Username string
	Password string
}

// Builder is a ProvisionerBuilder that creates a RackHD instance provisioner
type Builder struct {
	options Options
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("rackhd", pflag.PanicOnError)
	flags.StringVar(&b.options.Endpoint, "endpoint", "http://localhost:9090", "RackHD API Endpoint")
	flags.StringVar(&b.options.Username, "username", "admin", "RackHD Username")
	flags.StringVar(&b.options.Password, "password", "admin123", "RackHD Password")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin() (instance.Plugin, error) {
	mc := monorail.New(b.options.Endpoint)
	return NewInstancePlugin(mc, b.options.Username, b.options.Password), nil
}
