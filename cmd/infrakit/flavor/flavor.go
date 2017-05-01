package flavor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/flavor")

func init() {
	base.Register(Command)
}

// Command is the entry point of this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var flavorPlugin flavor.Plugin

	cmd := &cobra.Command{
		Use:   "flavor",
		Short: "Access flavor plugin",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	flavorPropertiesURL := cmd.PersistentFlags().String("properties", "", "Properties of the flavor plugin, a url")

	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
			return err
		}

		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		p, err := flavor_plugin.NewClient(plugin.Name(*name), endpoint.Address)
		if err != nil {
			return err
		}
		flavorPlugin = p

		cli.MustNotNil(flavorPlugin, "flavor plugin not found", "name", *name)
		return nil
	}

	logicalIDs := []string{}
	groupSize := uint(0)
	groupID := ""
	groupSequence := uint(0)

	addAllocationMethodFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringSliceVar(
			&logicalIDs,
			"logical-ids",
			[]string{},
			"Logical IDs to use as the Allocation method")
		cmd.Flags().UintVar(
			&groupSize,
			"size",
			0,
			"Group Size to use as the Allocation method")
	}
	allocationMethodFromFlags := func() group_types.AllocationMethod {
		ids := []instance.LogicalID{}
		for _, id := range logicalIDs {
			ids = append(ids, instance.LogicalID(id))
		}

		return group_types.AllocationMethod{
			Size:       groupSize,
			LogicalIDs: ids,
		}
	}

	indexFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVar(
			&groupID,
			"index-group",
			"",
			"ID of the group")
		cmd.Flags().UintVar(
			&groupSequence,
			"index-sequence",
			0,
			"Sequence number within the group")
	}

	indexFromFlags := func() group_types.Index {
		return group_types.Index{Group: group.ID(groupID), Sequence: groupSequence}
	}

	templateFlags, toJSON, _, processTemplate := base.TemplateProcessor(plugins)

	///////////////////////////////////////////////////////////////////////////////////
	// validate
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate a flavor configuration",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			view, err := processTemplate(*flavorPropertiesURL)
			if err != nil {
				return err
			}

			buff, err := toJSON([]byte(view))
			if err != nil {
				return err
			}

			return flavorPlugin.Validate(types.AnyBytes(buff), allocationMethodFromFlags())
		},
	}
	validate.Flags().AddFlagSet(templateFlags)
	addAllocationMethodFlags(validate)

	///////////////////////////////////////////////////////////////////////////////////
	// prepare
	prepare := &cobra.Command{
		Use:   "prepare <instance Spec template url>",
		Short: "Prepare provisioning inputs for an instance. Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			flavorProperties, err := processTemplate(*flavorPropertiesURL)
			if err != nil {
				return err
			}

			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
				toJSON,
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
				allocationMethodFromFlags(),
				indexFromFlags(),
			)
			if err != nil {
				return err
			}

			if buff, err := json.MarshalIndent(spec, "  ", "  "); err == nil {
				fmt.Println(string(buff))
			}
			return err
		},
	}
	prepare.Flags().AddFlagSet(templateFlags)
	addAllocationMethodFlags(prepare)
	indexFlags(prepare)

	///////////////////////////////////////////////////////////////////////////////////
	// healthy
	healthy := &cobra.Command{
		Use:   "healthy",
		Short: "checks if an instance is considered healthy",
	}
	tags := healthy.Flags().StringSlice("tags", []string{}, "Tags to filter")
	id := healthy.Flags().String("id", "", "ID of resource")
	logicalID := healthy.Flags().String("logical-id", "", "Logical ID of resource")
	healthy.Flags().AddFlagSet(templateFlags)
	healthy.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		flavorProperties, err := processTemplate(*flavorPropertiesURL)
		if err != nil {
			return err
		}

		filter := map[string]string{}
		for _, t := range *tags {
			p := strings.Split(t, "=")
			if len(p) == 2 {
				filter[p[0]] = p[1]
			} else {
				filter[p[0]] = ""
			}
		}

		desc := instance.Description{}
		if len(filter) > 0 {
			desc.Tags = filter
		}
		if *id != "" {
			desc.ID = instance.ID(*id)
		}
		if *logicalID != "" {
			logical := instance.LogicalID(*logicalID)
			desc.LogicalID = &logical
		}

		healthy, err := flavorPlugin.Healthy(types.AnyString(flavorProperties), desc)
		if err == nil {
			fmt.Printf("%v\n", healthy)
		}
		return err
	}

	cmd.AddCommand(
		validate,
		prepare,
		healthy,
	)

	return cmd
}
