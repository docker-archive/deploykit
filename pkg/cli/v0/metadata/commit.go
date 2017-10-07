package metadata

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Commit returns the Commit command
func Commit(name string, services *cli.Services) *cobra.Command {

	ls := &cobra.Command{
		Use:   "commit",
		Short: "Commit metadata updates",
	}

	ls.RunE = func(cmd *cobra.Command, args []string) error {

		metadataPlugin, err := LoadPlugin(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(metadataPlugin, "metadata plugin not found", "name", name)
		return nil
	}
	return ls
}
