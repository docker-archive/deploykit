package flavor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

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

	validate := &cobra.Command{
		Use:   "validate <flavor configuration file>",
		Short: "validate a flavor configuration",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			buff, err := ioutil.ReadFile(args[0])
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			return flavorPlugin.Validate(types.AnyBytes(buff), allocationMethodFromFlags())
		},
	}
	addAllocationMethodFlags(validate)
	cmd.AddCommand(validate)

	prepare := &cobra.Command{
		Use:   "prepare <flavor configuration file> <instance Spec JSON file>",
		Short: "prepare provisioning inputs for an instance",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 2 {
				cmd.Usage()
				os.Exit(1)
			}

			flavorProperties, err := ioutil.ReadFile(args[0])
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			buff, err := ioutil.ReadFile(args[1])
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			spec := instance.Spec{}
			if err := json.Unmarshal(buff, &spec); err != nil {
				return err
			}

			spec, err = flavorPlugin.Prepare(
				types.AnyBytes(flavorProperties),
				spec,
				allocationMethodFromFlags(),
				indexFromFlags(),
			)
			if err == nil {
				buff, err = json.MarshalIndent(spec, "  ", "  ")
				if err == nil {
					fmt.Println(string(buff))
				}
			}
			return err
		},
	}
	addAllocationMethodFlags(prepare)
	indexFlags(prepare)
	cmd.AddCommand(prepare)

	healthy := &cobra.Command{
		Use:   "healthy <flavor configuration file>",
		Short: "checks if an instance is considered healthy",
	}
	tags := healthy.Flags().StringSlice("tags", []string{}, "Tags to filter")
	id := healthy.Flags().String("id", "", "ID of resource")
	logicalID := healthy.Flags().String("logical-id", "", "Logical ID of resource")
	healthy.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		flavorProperties, err := ioutil.ReadFile(args[0])
		if err != nil {
			log.Error(err)
			os.Exit(1)
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

		healthy, err := flavorPlugin.Healthy(types.AnyBytes(flavorProperties), desc)
		if err == nil {
			fmt.Printf("%v\n", healthy)
		}
		return err
	}
	cmd.AddCommand(healthy)

	return cmd
}
