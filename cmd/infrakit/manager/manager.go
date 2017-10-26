package manager

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/cmd/infrakit/manager/schema"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/rpc/client"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/sergi/go-diff/diffmatchpatch"
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

						updatablePlugin = metadata_rpc.AdaptUpdatable(plugin.Name(name), rpcClient)
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

			commitEachGroup := func(name plugin.Name, gid group.ID, gspec group_types.Spec) error {
				endpoint, err := plugins().Find(name)
				if err != nil {
					return err
				}
				target, err := group_rpc.NewClient(endpoint.Address)
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
			pm, err := plugins().List()
			if err != nil {
				return err
			}

			for _, endpoint := range pm {
				rpcClient, err := client.New(endpoint.Address, manager.InterfaceSpec)
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

	///////////////////////////////////////////////////////////////////////////////////
	// change
	change := &cobra.Command{
		Use:   "change",
		Short: "Change returns the plugin configurations known by the manager",
	}
	vars := change.Flags().StringSlice("var", []string{}, "key=value pairs")
	commitChange := change.Flags().BoolP("commit", "c", false, "Commit changes")

	// This is the only interactive command.  We want to show the user the proposal, with the diff
	// and when the user accepts the change, call a commit.
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
		current, proposed, cas, err := updatablePlugin.Changes(changes)
		if err != nil {
			return err
		}
		currentBuff, err := current.MarshalYAML()
		if err != nil {
			return err
		}

		proposedBuff, err := proposed.MarshalYAML()
		if err != nil {
			return err
		}

		if *commitChange {
			fmt.Printf("Committing changes, hash=%s\n", cas)
		} else {
			fmt.Printf("Proposed changes, hash=%s\n", cas)
		}

		// Render the delta
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(currentBuff), string(proposedBuff), false)
		fmt.Println(dmp.DiffPrettyText(diffs))

		if *commitChange {
			return updatablePlugin.Commit(proposed, cas)
		}

		return nil
	}
	change.Flags().AddFlagSet(templateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// change-list
	changeList := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the changeable paths",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			all, err := types.ListAll(updatablePlugin, types.PathFromString("."))
			if err != nil {
				return err
			}

			types.SortPaths(all)
			for _, p := range all {
				fmt.Println(p.String())
			}
			return nil
		},
	}
	///////////////////////////////////////////////////////////////////////////////////
	// change-cat
	changeGet := &cobra.Command{
		Use:   "cat",
		Short: "Cat returns the current value at given path",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			path := types.PathFromString(args[0])
			any, err := updatablePlugin.Get(path)
			if err != nil {
				return err
			}

			fmt.Println(any.String())

			return nil
		},
	}
	change.AddCommand(changeList, changeGet)

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
