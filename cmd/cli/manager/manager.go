package manager

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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

	// This is the only interactive command.  We want to show the user the proposal, with the diff
	// and when the user accepts the change, call a commit.
	change.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		log.Info("applying changes")

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

		// render the proposal
		fmt.Printf("Proposed changes, hash=%s\n", cas)
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(currentBuff), string(proposedBuff), false)
		fmt.Println(dmp.DiffPrettyText(diffs))

		if *commitChange {
			// ask for final approval
			input := bufio.NewReader(os.Stdin)
			fmt.Fprintf(os.Stderr, "\n\nCommit? [y/n] ")
			text, _ := input.ReadString('\n')
			text = strings.Trim(text, " \t\n")

			agree, err := parseBool(text)
			if err != nil {
				return fmt.Errorf("not boolean %v", text)
			}
			if !agree {
				fmt.Fprintln(os.Stderr, "Not committing. Bye.")
				os.Exit(0)
			}

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

			types.Sort(all)
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

	cmd.AddCommand(commit, inspect, change)

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
