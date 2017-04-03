package group

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	group_plugin "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/group")

const (
	// DefaultGroupPluginName specifies the default name of the group plugin if name flag isn't specified.
	DefaultGroupPluginName = "group"
)

func init() {
	base.Register(Command)
}

// Command is the entrypoint to this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var groupPlugin group.Plugin

	cmd := &cobra.Command{
		Use:   "group",
		Short: "Access group plugin",
	}

	name := cmd.PersistentFlags().String("name", DefaultGroupPluginName, "Name of plugin")
	pretend := cmd.PersistentFlags().Bool("pretend", false, "Don't actually commit, only explain where appropriate")
	quiet := cmd.PersistentFlags().BoolP("quiet", "q", false, "Print rows without column headers")

	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
			return err
		}

		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		p, err := group_plugin.NewClient(endpoint.Address)
		if err != nil {
			return err
		}
		groupPlugin = p

		cli.MustNotNil(groupPlugin, "group plugin not found", "name", *name)
		return nil
	}

	///////////////////////////////////////////////////////////////////////////////////
	// commit
	tflags, toJSON, _, processTemplate := base.TemplateProcessor(plugins)
	commit := &cobra.Command{
		Use:   "commit <group configuration url>",
		Short: "Commit a group configuration. Read from stdin if url is '-'",
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

			spec := group.Spec{}
			if err := types.AnyString(view).Decode(&spec); err != nil {
				return err
			}

			details, err := groupPlugin.CommitGroup(spec, *pretend)
			if err == nil {
				if *pretend {
					fmt.Printf("Committing %s would involve: %s\n", spec.ID, details)
				} else {
					fmt.Printf("Committed %s: %s\n", spec.ID, details)
				}
			}
			return err
		},
	}
	commit.Flags().AddFlagSet(tflags)

	///////////////////////////////////////////////////////////////////////////////////
	// free
	free := &cobra.Command{
		Use:   "free <group ID>",
		Short: "Free a group from active monitoring, nondestructive",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			groupID := group.ID(args[0])
			err := groupPlugin.FreeGroup(groupID)
			if err == nil {
				fmt.Println("Freed", groupID)
			}
			return err
		},
	}

	///////////////////////////////////////////////////////////////////////////////////
	// describe
	describe := &cobra.Command{
		Use:   "describe <group ID>",
		Short: "Describe the live instances that make up a group",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			groupID := group.ID(args[0])
			desc, err := groupPlugin.DescribeGroup(groupID)

			if err == nil {
				if !*quiet {
					fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
				}
				for _, d := range desc.Instances {
					logical := "  -   "
					if d.LogicalID != nil {
						logical = string(*d.LogicalID)
					}

					printTags := []string{}
					for k, v := range d.Tags {
						printTags = append(printTags, fmt.Sprintf("%s=%s", k, v))
					}
					sort.Strings(printTags)

					fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, strings.Join(printTags, ","))
				}
			}
			return err
		},
	}

	///////////////////////////////////////////////////////////////////////////////////
	// inspect
	inspectTemplateFlags, _, fromJSON, _ := base.TemplateProcessor(plugins)
	inspect := &cobra.Command{
		Use:   "inspect <group ID>",
		Short: "Insepct a group. Returns the raw configuration associated with a group",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			groupID := group.ID(args[0])
			specs, err := groupPlugin.InspectGroups()

			if err == nil {

				for _, spec := range specs {
					if spec.ID == groupID {
						data, err := json.MarshalIndent(spec, "", "  ")
						if err != nil {
							return err
						}

						data, err = fromJSON(data)
						if err != nil {
							return err
						}

						fmt.Println(string(data))

						return nil
					}
				}

				return fmt.Errorf("Group %s is not being watched", groupID)
			}
			return err
		},
	}
	inspect.Flags().AddFlagSet(inspectTemplateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	//  destroy
	destroy := &cobra.Command{
		Use:   "destroy <group ID>",
		Short: "Destroy a group",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			groupID := group.ID(args[0])
			err := groupPlugin.DestroyGroup(groupID)

			if err == nil {
				fmt.Println("destroy", groupID, "initiated")
			}
			return err
		},
	}

	///////////////////////////////////////////////////////////////////////////////////
	//  ls
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List groups",
		RunE: func(cmd *cobra.Command, args []string) error {

			groups, err := groupPlugin.InspectGroups()
			if err == nil {
				if !*quiet {
					fmt.Printf("%s\n", "ID")
				}
				for _, g := range groups {
					fmt.Printf("%s\n", g.ID)
				}
			}

			return err
		},
	}

	cmd.AddCommand(
		commit,
		free,
		describe,
		inspect,
		destroy,
		ls,
	)

	return cmd
}
