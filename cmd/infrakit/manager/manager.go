package manager

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/cmd/infrakit/manager/schema"

	"github.com/docker/infrakit/pkg/cli"
	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/manager")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(scope scope.Scope) *cobra.Command {

	services := cli.NewServices(scope)

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
			pm, err := scope.Plugins().List()
			if err != nil {
				return err
			}

			for name, endpoint := range pm {

				rpcClient, err := client.New(endpoint.Address, stack.InterfaceSpec)
				if err == nil {

					m := manager_rpc.Adapt(rpcClient)

					isLeader, err := m.IsLeader()
					if err != nil {
						return err
					}

					log.Debug("Found manager", "name", name, "leader", isLeader)
					if isLeader {

						pn := plugin.Name(name)
						groupPlugin = group_rpc.Adapt(pn, rpcClient)
						groupPluginName = name

						log.Debug("Found manager", "name", name, "addr", endpoint.Address)

						updatablePlugin = metadata_rpc.AdaptUpdatable(pn, rpcClient)
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

			view, err := services.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return services.ProcessTemplate(args[0]) },
				services.ToJSON,
			)
			if err != nil {
				return err
			}

			commitEachGroup := func(name plugin.Name, gid group.ID, gspec group_types.Spec) error {
				endpoint, err := scope.Plugins().Find(name)
				if err != nil {
					return err
				}
				target, err := group_rpc.NewClient(name, endpoint.Address)
				log.Debug("commit", "plugin", name, "address", endpoint.Address, "err", err, "gspec", gspec)

				if err != nil {
					return err
				}

				any, err := types.AnyValue(gspec)
				if err != nil {
					return err
				}

				plan, err := target.CommitGroup(group.Spec{
					ID:         gid,
					Properties: any,
				}, *pretend)

				if err != nil {
					return err
				}

				fmt.Println("Group", gid, "with plugin", name, "plan:", plan)
				return nil
			}
			return schema.ParseInputSpecs([]byte(view), commitEachGroup)
		},
	}
	commit.Flags().AddFlagSet(services.ProcessTemplateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// inspect
	inspect := &cobra.Command{
		Use: "inspect",
		Short: "DEPRECATED - Please use `infrakit " + local.InfrakitHost() +
			" <stackname> specs` to show the global specs enforced by the stack (manager)",

		RunE: func(cmd *cobra.Command, args []string) error {

			fmt.Println("**** DEPRECATED ****")
			fmt.Println("Please use `infrakit " + local.InfrakitHost() +
				" <stackname> specs` to show the global specs enforced by the stack (manager)")

			cmd.Usage()
			os.Exit(1)
			return nil
		},
	}
	inspect.Flags().AddFlagSet(services.ProcessTemplateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// leader

	leader := &cobra.Command{
		Use:   "leader",
		Short: "Leader returns the leadership information",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}
			// Scan for a manager
			pm, err := scope.Plugins().List()
			if err != nil {
				return err
			}

			for _, endpoint := range pm {
				rpcClient, err := client.New(endpoint.Address, stack.InterfaceSpec)
				if err == nil {

					m := manager_rpc.Adapt(rpcClient)

					isleader := "unknown"
					if l, err := m.IsLeader(); err == nil {
						if l {
							isleader = "true"
						} else {
							isleader = "false"
						}
					} else {
						log.Warn("error determining leader", "err", err)
					}
					fmt.Printf("IsLeader       : %v\n", isleader)

					location := "unknown"
					if l, err := m.LeaderLocation(); err == nil {
						location = l.String()
					} else {
						log.Warn("error getting location of leader", "err", err)
					}
					fmt.Printf("LeaderLocation : %v\n", location)

					return nil
				}
			}

			fmt.Println("no manager found")
			return nil
		},
	}

	cmd.AddCommand(commit, inspect, leader)

	return cmd
}

func parseBool(text string) (bool, error) {
	agree, err := strconv.ParseBool(text)
	if err == nil {
		return agree, nil
	}
	switch strings.ToLower(text) {
	case "yes", "ok", "y":
		return true, nil
	case "no", "nope", "n":
		return false, nil
	}
	return false, err
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
			Value: types.AnyYAMLMust([]byte(value)),
		}

		changes = append(changes, change)
	}
	return changes, nil
}
