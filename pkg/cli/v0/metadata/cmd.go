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
	cli.Register(metadata.UpdatableInterfaceSpec,
		[]cli.CmdBuilder{
			MetadataUpdatable,
		})
}

// loadPlugin loads the typed plugin
func loadPlugin(plugins discovery.Plugins, name string) (metadata.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return metadata_rpc.NewClient(endpoint.Address)
}

// loadPluginUpdatable loads the typed plugin
func loadPluginUpdatable(plugins discovery.Plugins, name string) (metadata.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return metadata_rpc.NewClientUpdatable(endpoint.Address)
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

// MetadataUpdatable returns the metadata root command
func MetadataUpdatable(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Access metadata of " + name,
	}

	cmd.AddCommand(
		Ls(name, services),
		Cat(name, services),
		Change(name, services),
		Commit(name, services),
	)

	return cmd
}
