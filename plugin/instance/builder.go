package instance

import (
	"github.com/codedellemc/gorackhd/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	apiClient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/pflag"
)

type options struct {
	endpoint  string
	transport string
}

// Builder is a ProvisionerBuilder that creates a RackHD instance provisioner
type Builder struct {
	client  *apiClient.Runtime
	options options
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("rackhd", pflag.PanicOnError)
	flags.StringVar(&b.options.endpoint, "endpoint", "localhost:9090", "RackHD API Endpoint")
	flags.StringVar(&b.options.transport, "transport", "http", "Transport Scheme for RackHD Client")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin() (instance.Plugin, error) {
	if b.client == nil {
		b.client = apiClient.New(b.options.endpoint, "/api/2.0",
			[]string{b.options.transport})
	}
	return NewInstancePlugin(client.New(b.client, strfmt.Default)), nil
}
