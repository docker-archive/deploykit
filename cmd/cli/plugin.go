package main

import (
	"fmt"
	"github.com/docker/infrakit/discovery"
	"github.com/spf13/cobra"
)

func pluginCommand(pluginDir func() *discovery.Dir) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List available plugins",
		RunE: func(c *cobra.Command, args []string) error {
			entries, err := pluginDir().List()
			if err != nil {
				return err
			}

			fmt.Println("Plugins:")
			fmt.Printf("%-20s\t%-s\n", "NAME", "LISTEN")
			for k, v := range entries {
				fmt.Printf("%-20s\t%-s\n", k, v.String())
			}

			return nil
		},
	})

	return cmd
}
