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
			Keys,
			Cat,
		})
	cli.Register(metadata.UpdatableInterfaceSpec,
		[]cli.CmdBuilder{
			Keys,
			Cat,
			Change,
		})
}

// loadPlugin loads the typed plugin
func loadPlugin(plugins discovery.Plugins, name string) (metadata.Plugin, error) {
	pluginName := plugin.Name(name)
	endpoint, err := plugins.Find(pluginName)
	if err != nil {
		return nil, err
	}
	return metadata_rpc.NewClient(pluginName, endpoint.Address)
}

// loadPluginUpdatable loads the typed plugin
func loadPluginUpdatable(plugins discovery.Plugins, name string) (metadata.Updatable, error) {
	pluginName := plugin.Name(name)
	endpoint, err := plugins.Find(pluginName)
	if err != nil {
		return nil, err
	}
	return metadata_rpc.NewClientUpdatable(pluginName, endpoint.Address)
}

// Metadata returns the metadata root command
func Metadata(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Access metadata of " + name,
	}

	cmd.AddCommand(
		Keys(name, services),
		Cat(name, services),
	)

	return cmd
}

// Updatable returns the metadata root command
func Updatable(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Access metadata of " + name,
	}

	cmd.AddCommand(
		Keys(name, services),
		Cat(name, services),
		Change(name, services),
	)

	return cmd
}
