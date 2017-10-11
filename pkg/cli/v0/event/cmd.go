package event

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/v1/event")

func init() {
	cli.Register(event.InterfaceSpec,
		[]cli.CmdBuilder{
			Ls,
			Tail,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (event.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return event_rpc.NewClient(endpoint.Address)
}

// Event returns the event root command
func Event(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "event",
		Short: "Access events of " + name,
	}

	cmd.AddCommand(
		Ls(name, services),
		Tail(name, services),
	)

	return cmd
}
