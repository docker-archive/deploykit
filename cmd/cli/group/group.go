package group

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	group_plugin "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

const (
	// DefaultGroupPluginName specifies the default name of the group plugin if name flag isn't specified.
	DefaultGroupPluginName = "group"
)

// Command is the entrypoint to this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var groupPlugin group.Plugin

	cmd := &cobra.Command{
		Use:   "group",
		Short: "Access group plugin",
	}
	name := cmd.PersistentFlags().String("name", DefaultGroupPluginName, "Name of plugin")
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

	commit := cobra.Command{
		Use:   "commit <group configuration>",
		Short: "commit a group configuration",
	}
	pretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")
	commit.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		buff, err := ioutil.ReadFile(args[0])
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}

		spec := group.Spec{}
		if err := json.Unmarshal(buff, &spec); err != nil {
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
	}
	cmd.AddCommand(&commit)

	cmd.AddCommand(&cobra.Command{
		Use:   "free <group ID>",
		Short: "free a group from active monitoring, nondestructive",
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
	})

	describe := &cobra.Command{
		Use:   "describe <group ID>",
		Short: "describe the live instances that make up a group",
	}
	quietDescribe := describe.Flags().BoolP("quiet", "q", false, "Print rows without column headers")
	describe.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		groupID := group.ID(args[0])
		desc, err := groupPlugin.DescribeGroup(groupID)

		if err == nil {
			if !*quietDescribe {
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
	}
	cmd.AddCommand(describe)

	cmd.AddCommand(&cobra.Command{
		Use:   "inspect <group ID>",
		Short: "return the raw configuration associated with a group",
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

						fmt.Println(string(data))

						return nil
					}
				}

				return fmt.Errorf("Group %s is not being watched", groupID)
			}
			return err
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "destroy <group ID>",
		Short: "destroy a group",
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
	})

	describeGroups := &cobra.Command{
		Use:   "ls",
		Short: "list groups",
	}
	quietls := describeGroups.Flags().BoolP("quiet", "q", false, "Print rows without column headers")
	describeGroups.RunE = func(cmd *cobra.Command, args []string) error {

		groups, err := groupPlugin.InspectGroups()
		if err == nil {
			if !*quietls {
				fmt.Printf("%s\n", "ID")
			}
			for _, g := range groups {
				fmt.Printf("%s\n", g.ID)
			}
		}

		return err
	}
	cmd.AddCommand(describeGroups)

	return cmd
}
