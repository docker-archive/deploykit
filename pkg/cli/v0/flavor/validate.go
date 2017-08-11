package flavor

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Validate returns the validate command
func Validate(name string, services *cli.Services) *cobra.Command {
	logicalIDs := []string{}
	groupSize := uint(0)

	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate a flavor configuration",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			flavorPlugin, err := LoadPlugin(services.Plugins(), name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(flavorPlugin, "instance plugin not found", "name", name)

			view, err := services.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return services.ProcessTemplate(args[0]) },
				services.ToJSON,
			)
			if err != nil {
				return err
			}

			err = flavorPlugin.Validate(types.AnyString(view), allocationMethodFromFlags(&logicalIDs, &groupSize))
			if err == nil {
				fmt.Println("validate:ok")
			}
			return err
		},
	}

	validate.Flags().StringSliceVar(
		&logicalIDs,
		"logical-ids",
		[]string{},
		"Logical IDs to use as the Allocation method")
	validate.Flags().UintVar(
		&groupSize,
		"size",
		0,
		"Group Size to use as the Allocation method")
	validate.Flags().AddFlagSet(services.ProcessTemplateFlags)

	return validate
}

func allocationMethodFromFlags(logicalIDs *[]string, groupSize *uint) group_types.AllocationMethod {
	ids := []instance.LogicalID{}
	for _, id := range *logicalIDs {
		ids = append(ids, instance.LogicalID(id))
	}

	return group_types.AllocationMethod{
		Size:       *groupSize,
		LogicalIDs: ids,
	}
}
