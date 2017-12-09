package group

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Inspect returns the Inspect command
func Inspect(name string, services *cli.Services) *cobra.Command {

	inspect := &cobra.Command{
		Use:   "inspect <group ID>",
		Short: "Inspect a group. Returns the raw configuration associated with a group",
		RunE: func(cmd *cobra.Command, args []string) error {

			pluginName := plugin.Name(name)
			_, gid := pluginName.GetLookupAndType()
			if gid == "" {
				if len(args) < 1 {
					cmd.Usage()
					os.Exit(1)
				} else {
					gid = args[0]
				}
			}

			groupPlugin, err := services.Scope.Group(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(gid)
			specs, err := groupPlugin.InspectGroups()

			if err == nil {

				for _, spec := range specs {
					if spec.ID == groupID {
						return services.Output(os.Stdout, spec, nil)
					}
				}

				return fmt.Errorf("Group %s is not being watched", groupID)
			}
			return err
		},
	}
	inspect.Flags().AddFlagSet(services.OutputFlags)
	return inspect
}
