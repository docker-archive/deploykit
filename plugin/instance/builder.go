package instance

import (
	"github.com/codedellemc/infrakit.rackhd/monorail"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/go-openapi/runtime"
	rc "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/pflag"
)

type options struct {
	endpoint string
	protocol string
}

// Builder is a ProvisionerBuilder that creates a RackHD instance provisioner
type Builder struct {
	transport runtime.ClientTransport
	options   options
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("rackhd", pflag.PanicOnError)
	flags.StringVar(&b.options.endpoint, "endpoint", "localhost:9090", "RackHD API Endpoint")
	flags.StringVar(&b.options.protocol, "protocol", "http", "Protocol for RackHD Client <HTTP, HTTPS>")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin() (instance.Plugin, error) {
	b.transport = rc.New(b.options.endpoint, "/api/2.0", []string{b.options.protocol})
	mc := monorail.New(b.transport, strfmt.Default)
	return NewInstancePlugin(mc), nil
}
