package group

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

// DestroyInstances returns the DestroyInstances command
func DestroyInstances(name string, services *cli.Services) *cobra.Command {

	destroy := &cobra.Command{
		Use:   "destroy-instances <groupID> <instance ID>...",
		Short: "Destroy a group's instances",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) < 2 {
				cmd.Usage()
				os.Exit(1)
			}

			groupPlugin, err := LoadPlugin(services.Plugins(), name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(args[0])

			instances := []instance.ID{}
			for i := 1; i < len(args); i++ {
				instances = append(instances, instance.ID(i))
			}

			err = groupPlugin.DestroyInstances(groupID, instances)
			if err != nil {
				return err
			}

			fmt.Println("DestroyInstances", groupID, "initiated for instances", instances)
			return nil
		},
	}
	return destroy
}
