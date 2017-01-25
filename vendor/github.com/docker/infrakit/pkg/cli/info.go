package cli

import (
	"encoding/json"
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/spf13/cobra"
)

// InfoCommand creates a cobra Command that prints build version information.
func InfoCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "print plugin info",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		infoClient := client.NewPluginInfoClient(endpoint.Address)
		info, err := infoClient.GetInfo()
		if err != nil {
			return err
		}

		buff, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(buff))
		return nil
	}
	return cmd
}
