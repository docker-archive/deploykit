package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/docker/infrakit/discovery"
	"github.com/docker/infrakit/spi/group"
	group_plugin "github.com/docker/infrakit/spi/http/group"
	"github.com/spf13/cobra"
)

func groupPluginCommand(pluginDir func() *discovery.Dir) *cobra.Command {

	name := ""
	var groupPlugin group.Plugin

	cmd := &cobra.Command{
		Use:   "group",
		Short: "Access group plugin",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {

			assertNotNil("cannot lookup", pluginDir())

			callable, err := pluginDir().PluginByName(name)
			if err != nil {
				return err
			}
			groupPlugin = group_plugin.PluginClient(callable)

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&name, "name", name, "Name of plugin")

	watch := &cobra.Command{
		Use:   "watch <group configuration>",
		Short: "watch a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			buff := getInput(args)
			spec := group.Spec{}
			err := json.Unmarshal(buff, &spec)
			if err != nil {
				return err
			}

			err = groupPlugin.WatchGroup(spec)
			if err == nil {
				fmt.Println("watching", spec.ID)
			}
			return err
		},
	}

	unwatch := &cobra.Command{
		Use:   "unwatch [group ID]",
		Short: "unwatch a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			if len(args) == 0 {
				return errors.New("missing id")
			}

			groupID := group.ID(args[0])
			err := groupPlugin.UnwatchGroup(groupID)

			if err == nil {
				fmt.Println("unwatched", groupID)
			}
			return err
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect [group ID]",
		Short: "inspect a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			if len(args) == 0 {
				return errors.New("missing id")
			}

			groupID := group.ID(args[0])
			desc, err := groupPlugin.InspectGroup(groupID)

			if err == nil {
				fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
				for _, d := range desc.Instances {
					logical := "  -   "
					if d.LogicalID != nil {
						logical = string(*d.LogicalID)
					}
					tagstr := ""
					for k, v := range d.Tags {
						sep := ""
						if tagstr != "" {
							sep = ","
						}
						tagstr = tagstr + sep + fmt.Sprintf("%s=%s", k, v)
					}
					fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, tagstr)
				}
			}
			return err
		},
	}

	describe := &cobra.Command{
		Use:   "describe",
		Short: "describe update (describe - or describe filename)",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			buff := getInput(args)
			spec := group.Spec{}
			err := json.Unmarshal(buff, &spec)
			if err != nil {
				return err
			}

			desc, err := groupPlugin.DescribeUpdate(spec)
			if err == nil {
				fmt.Println(spec.ID, ":", desc)
			}
			return err
		},
	}

	update := &cobra.Command{
		Use:   "update [group configuration]",
		Short: "update group (update < file or update filename)",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			buff := getInput(args)
			spec := group.Spec{}
			err := json.Unmarshal(buff, &spec)
			if err != nil {
				return err
			}

			err = groupPlugin.UpdateGroup(spec)
			if err == nil {
				fmt.Println("update", spec.ID, "initiated")
			}
			return err
		},
	}

	stop := &cobra.Command{
		Use:   "stop [group ID]",
		Short: "stop updating a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			if len(args) == 0 {
				return errors.New("missing id")
			}

			groupID := group.ID(args[0])
			err := groupPlugin.StopUpdate(groupID)

			if err == nil {
				fmt.Println("update", groupID, "stopped")
			}
			return err
		},
	}

	destroy := &cobra.Command{
		Use:   "destroy [group ID]",
		Short: "destroy a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", groupPlugin)

			if len(args) == 0 {
				return errors.New("missing id")
			}

			groupID := group.ID(args[0])
			err := groupPlugin.DestroyGroup(groupID)

			if err == nil {
				fmt.Println("destroy", groupID, "initiated")
			}
			return err
		},
	}

	cmd.AddCommand(watch, unwatch, inspect, describe, update, stop, destroy)

	return cmd
}
