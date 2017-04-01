package manager

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	group_plugin "github.com/docker/infrakit/pkg/rpc/group"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/spi/group"
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

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Access the manager",
	}
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
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

				log.Info("Found manager", "name", name, "leader", isLeader)
				if isLeader {

					groupPlugin = group_plugin.Adapt(rpcClient)
					groupPluginName = name

					log.Info("Found manager", "name", name, "addr", endpoint.Address)

					break
				}
			}
		}
		return nil
	}

	commit := cobra.Command{
		Use:   "commit <template_URL>",
		Short: "commit a multi-group configuration, as specified by the URL",
	}

	pretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")

	tflags, processTemplate := base.TemplateProcessor(plugins)
	commit.Flags().AddFlagSet(tflags)
	commit.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		templateURL := args[0]

		view, err := processTemplate(templateURL)
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
			target, err := group_plugin.NewClient(endpoint.Address)

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
	}

	inspect := cobra.Command{
		Use:   "inspect",
		Short: "inspect returns the plugin configurations known by the manager",
	}
	inspect.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		specs, err := groupPlugin.InspectGroups()
		if err != nil {
			return err
		}

		// the format is plugin.Spec
		out := []plugin.Spec{}
		for _, spec := range specs {

			any, err := types.AnyValue(spec)
			if err != nil {
				return err
			}

			out = append(out, plugin.Spec{
				Plugin:     plugin.Name(groupPluginName),
				Properties: any,
			})
		}

		view, err := types.AnyValue(out)
		if err != nil {
			return err
		}
		fmt.Println(view.String())

		return nil
	}

	cmd.AddCommand(&commit, &inspect)

	return cmd
}
