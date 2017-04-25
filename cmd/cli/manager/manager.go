package manager

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/manager")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var groupPlugin group.Plugin
	var groupPluginName string

	var updatablePlugin metadata.Updatable
	var updatablePluginName string

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Access the manager",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			if err := cli.EnsurePersistentPreRunE(c); err != nil {
				return err
			}

			// Scan for a manager
			pm, err := plugins().List()
			if err != nil {
				return err
			}

			for name, endpoint := range pm {

				rpcClient, err := client.New(endpoint.Address, manager.InterfaceSpec)
				if err == nil {

					m := manager_rpc.Adapt(rpcClient)

					isLeader, err := m.IsLeader()
					if err != nil {
						return err
					}

					log.Debug("Found manager", "name", name, "leader", isLeader)
					if isLeader {

						groupPlugin = group_rpc.Adapt(rpcClient)
						groupPluginName = name

						log.Debug("Found manager", "name", name, "addr", endpoint.Address)

						updatablePlugin = metadata_rpc.AdaptUpdatable(rpcClient)
						updatablePluginName = name

						log.Debug("Found updatable", "name", name, "addr", endpoint.Address)
						break
					}
				}
			}

			// We need to enforce the requirement that we run on a leader node.
			if groupPlugin == nil {
				return fmt.Errorf("Cannot perform manager operations on a non-leader node")
			}
			return nil
		},
	}
	pretend := cmd.PersistentFlags().Bool("pretend", false, "Don't actually make changes; explain where appropriate")

	templateFlags, toJSON, fromJSON, processTemplate := base.TemplateProcessor(plugins)

	///////////////////////////////////////////////////////////////////////////////////
	// commit
	commit := &cobra.Command{
		Use:   "commit <template_URL>",
		Short: "Commit a multi-group configuration, as specified by the URL.  Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
				toJSON,
			)
			if err != nil {
				return err
			}

			// In any case, the view should be in JSON format

			// Treat this as an Any and then convert
			any := types.AnyString(view)

			groups := []plugin.Spec{}
			err = any.Decode(&groups)
			if err != nil {
				log.Warn("Error parsing the template for plugin specs.")
				return err
			}

			// Check the list of plugins
			for _, gp := range groups {

				endpoint, err := plugins().Find(gp.Plugin)
				if err != nil {
					return err
				}

				// unmarshal the group spec
				spec := group.Spec{}
				if gp.Properties != nil {
					err = gp.Properties.Decode(&spec)
					if err != nil {
						return err
					}
				}

				// TODO(chungers) -- we need to enforce and confirm the type of this.
				// Right now we assume the RPC endpoint is indeed a group.
				target, err := group_rpc.NewClient(endpoint.Address)

				log.Debug("commit", "plugin", gp.Plugin, "address", endpoint.Address, "err", err, "spec", spec)

				if err != nil {
					return err
				}

				plan, err := target.CommitGroup(spec, *pretend)
				if err != nil {
					return err
				}

				fmt.Println("Group", spec.ID, "with plugin", gp.Plugin, "plan:", plan)
			}

			return nil
		},
	}
	commit.Flags().AddFlagSet(templateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// inspect
	inspect := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect returns the plugin configurations known by the manager",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			out, err := getGlobalConfig(groupPlugin, groupPluginName)
			if err != nil {
				return err
			}

			view, err := types.AnyValue(out)
			if err != nil {
				return err
			}

			buff, err := fromJSON(view.Bytes())
			if err != nil {
				return err
			}

			fmt.Println(string(buff))

			return nil
		},
	}
	inspect.Flags().AddFlagSet(templateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// change
	change := &cobra.Command{
		Use:   "change",
		Short: "Change returns the plugin configurations known by the manager",
	}
	vars := change.Flags().StringSliceP("var", "v", []string{}, "key=value pairs")
	commitChange := change.Flags().BoolP("commit", "c", false, "Commit changes")
	casFlag := change.Flags().StringP("version", "h", "", "CAS for commit")

	change.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		// get the changes
		changes, err := changeSet(*vars)
		if err != nil {
			return err
		}

		proposed, cas, err := updatablePlugin.Changes(changes)
		if err != nil {
			return err
		}

		if *commitChange {

			if *casFlag != "" && *casFlag != cas {
				// this case here the user provided a wrong cas that's not
				// matched to the input changes
				return fmt.Errorf("The cas / version value does not match.")
			}

			return updatablePlugin.Commit(proposed, cas)
		}

		return nil
	}

	cmd.AddCommand(commit, inspect, change)

	return cmd
}

func getGlobalConfig(groupPlugin group.Plugin, groupPluginName string) ([]plugin.Spec, error) {
	specs, err := groupPlugin.InspectGroups()
	if err != nil {
		return nil, err
	}

	// the format is plugin.Spec
	out := []plugin.Spec{}
	for _, spec := range specs {

		any, err := types.AnyValue(spec)
		if err != nil {
			return nil, err
		}

		out = append(out, plugin.Spec{
			Plugin:     plugin.Name(groupPluginName),
			Properties: any,
		})
	}
	return out, nil
}

// changeSet returns a set of changes from the input pairs of path / value
func changeSet(kvPairs []string) ([]metadata.Change, error) {
	changes := []metadata.Change{}

	for _, kv := range kvPairs {

		parts := strings.SplitN(kv, "=", 2)
		key := strings.Trim(parts[0], " \t\n")
		value := strings.Trim(parts[1], " \t\n")

		change := metadata.Change{
			Path:  types.PathFromString(key),
			Value: types.AnyString(value),
		}

		changes = append(changes, change)
	}
	return changes, nil
}
