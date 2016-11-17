package main

import (
	"fmt"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/spf13/cobra"
)

func pluginCommand(plugins func() discovery.Plugins) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	ls := cobra.Command{
		Use:   "ls",
		Short: "List available plugins",
	}
	quiet := ls.Flags().BoolP("quiet", "q", false, "Print rows without column headers")
	ls.RunE = func(c *cobra.Command, args []string) error {
		entries, err := plugins().List()
		if err != nil {
			return err
		}

		if !*quiet {
			fmt.Printf("%-20s\t%-s\n", "NAME", "LISTEN")
		}
		for k, v := range entries {
			fmt.Printf("%-20s\t%-s\n", k, v.Address)
		}

		return nil
	}

	cmd.AddCommand(&ls)

	return cmd
}
