package instance

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

// Destroy returns the destroy command
func Destroy(name string, services *cli.Services) *cobra.Command {

	destroy := &cobra.Command{
		Use:   "destroy <instance ID>...",
		Short: "Destroy the instance",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) < 1 {
				cmd.Usage()
				os.Exit(1)
			}

			instancePlugin, err := services.Scope.Instance(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(instancePlugin, "instance plugin not found", "name", name)

			for _, a := range args {

				instanceID := instance.ID(a)
				err := instancePlugin.Destroy(instanceID, instance.Termination)

				if err != nil {
					return err
				}
				fmt.Println("destroyed", instanceID)
			}
			return nil
		},
	}
	return destroy
}
