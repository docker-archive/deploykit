package group

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

// DestroyInstances returns the DestroyInstances command
func DestroyInstances(name string, services *cli.Services) *cobra.Command {

	destroy := &cobra.Command{
		Use:   "destroy-instance <instance ID>...",
		Short: "Destroy a group's instances",
		RunE: func(cmd *cobra.Command, args []string) error {

			targets := args
			pluginName := plugin.Name(name)
			_, gid := pluginName.GetLookupAndType()
			if gid == "" {
				if len(args) < 1 {
					cmd.Usage()
					os.Exit(1)
				} else {
					gid = args[0]
					targets = args[1:]
				}
			}

			groupPlugin, err := services.Scope.Group(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(gid)

			instances := []instance.ID{}
			for _, a := range targets {
				instances = append(instances, instance.ID(a))
			}

			err = groupPlugin.DestroyInstances(groupID, instances)
			if err != nil {
				return err
			}

			fmt.Println("Initiated.")
			return nil
		},
	}
	return destroy
}
