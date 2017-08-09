package flavor

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Prepare returns the prepare command
func Prepare(name string, services *cli.Services) *cobra.Command {
	logicalIDs := []string{}
	groupSize := uint(0)
	groupID := ""
	groupSequence := uint(0)
	flavorPropertiesURL := ""

	prepare := &cobra.Command{
		Use:   "prepare <instance Spec template url | - >",
		Short: "Prepare provisioning inputs for an instance. Read from stdin if url is '-'",
	}

	prepare.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		flavorPlugin, err := LoadPlugin(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(flavorPlugin, "flavor plugin not found", "name", name)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		flavorProperties, err := services.ProcessTemplate(flavorPropertiesURL)
		if err != nil {
			return err
		}

		view, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		spec := instance.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		spec, err = flavorPlugin.Prepare(
			types.AnyString(flavorProperties),
			spec,
			allocationMethodFromFlags(&logicalIDs, &groupSize),
			indexFromFlags(&groupID, &groupSequence),
		)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, spec, nil)
	}

	prepare.Flags().String("properties", "", "Properties of the flavor plugin, a url")
	prepare.Flags().StringSliceVar(
		&logicalIDs,
		"logical-ids",
		[]string{},
		"Logical IDs to use as the Allocation method")
	prepare.Flags().UintVar(
		&groupSize,
		"size",
		0,
		"Group Size to use as the Allocation method")
	prepare.Flags().AddFlagSet(services.ProcessTemplateFlags)

	prepare.Flags().StringVar(
		&groupID,
		"index-group",
		"",
		"ID of the group")
	prepare.Flags().UintVar(
		&groupSequence,
		"index-sequence",
		0,
		"Sequence number within the group")
	return prepare
}

func indexFromFlags(groupID *string, groupSequence *uint) group_types.Index {
	return group_types.Index{Group: group.ID(*groupID), Sequence: *groupSequence}
}
