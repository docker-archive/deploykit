package metadata

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/v1/metadata")

func init() {
	cli.Register(metadata.InterfaceSpec,
		[]cli.CmdBuilder{
			Metadata,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (metadata.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return metadata_rpc.NewClient(endpoint.Address)
}

// Metadata returns the metadata root command
func Metadata(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Access metadata of " + name,
	}

	cmd.AddCommand(
		Ls(name, services),
		Cat(name, services),
	)

	return cmd
}
