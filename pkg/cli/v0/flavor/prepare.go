package flavor

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
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
	logicalID := ""
	viewTemplateURL := ""

	prepare := &cobra.Command{
		Use:   "prepare < flavor properties template URL | - >",
		Short: "Prepare provisioning inputs for an instance. Read from stdin if url is '-'",
	}

	prepare.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		flavorPlugin, err := services.Scope.Flavor(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(flavorPlugin, "flavor plugin not found", "name", name)

		view, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		spec := instance.Spec{}
		if logicalID != "" {
			lid := instance.LogicalID(logicalID)
			spec.LogicalID = &lid
		}
		spec, err = flavorPlugin.Prepare(
			types.AnyString(view),
			spec,
			allocationMethodFromFlags(&logicalIDs, &groupSize),
			indexFromFlags(&groupID, &groupSequence),
		)
		if err != nil {
			return err
		}

		var defaultView func(w io.Writer, v interface{}) error
		if viewTemplateURL != "" {
			defaultView = func(w io.Writer, v interface{}) error {

				rendered, err := services.ProcessTemplate(viewTemplateURL, spec)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "%s", rendered)
				return nil
			}
		}

		return services.Output(os.Stdout, spec, defaultView)
	}

	prepare.Flags().AddFlagSet(services.ProcessTemplateFlags)
	prepare.Flags().StringSliceVar(&logicalIDs, "logical-ids", []string{}, "Logical IDs to use as the Allocation method")
	prepare.Flags().UintVar(&groupSize, "size", 0, "Group Size to use as the Allocation method")
	prepare.Flags().StringVar(&logicalID, "logical-id", "", "Logical ID of instance to provision")
	prepare.Flags().StringVar(&groupID, "index-group", "", "ID of the group")
	prepare.Flags().StringVar(&viewTemplateURL, "view", "", "Template to apply to result for rendering")
	prepare.Flags().UintVar(&groupSequence, "index-sequence", 0, "Sequence number within the group")
	return prepare
}

func indexFromFlags(groupID *string, groupSequence *uint) group.Index {
	return group.Index{Group: group.ID(*groupID), Sequence: *groupSequence}
}
