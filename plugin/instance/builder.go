package instance

import(
  "github.com/codedellemc/gorackhd/client"
  httpclient "github.com/go-openapi/runtime/client"
  "github.com/go-openapi/strfmt"
  "github.com/spf13/pflag"
)

type options struct {
    endpoint  string
    transport string
}

type Builder struct {
    HttpClient httpclient
    options options
}

func (b *Builder) BuildInstancePlugin() (instance.Plugin, error) {
    if b.HttpClient == nil {
        b.HttpClient = httpclient.New(b.options.endpoint, "/api/2.0",
                        []string{b.options.transport})
    }
    return NewInstancePlugin(client.New(b.HttpClient, strfmt.Default)), nil
}

func (b *Builder) Flags() *pflag.FlagSet {
    flags := pflag.NewFlagSet("rackhd", pflag.PanicOnError)
    flags.StringVar(&b.options.endpoint, "endpoint", "localhost:9090", "RackHD API Endpoint")
    flags.StringVar(&b.options.transport, "transport", "http", "Transport Scheme for RackHD Client")
    return flags
}
