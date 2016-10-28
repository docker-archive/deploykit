package main

import (
	"fmt"
	"github.com/docker/infrakit/discovery"
	"github.com/spf13/cobra"
)

func pluginCommand(plugins func() discovery.Plugins) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	var quiet bool
	ls := cobra.Command{
		Use:   "ls",
		Short: "List available plugins",
		RunE: func(c *cobra.Command, args []string) error {
			entries, err := plugins().List()
			if err != nil {
				return err
			}

			if !quiet {
				fmt.Printf("%-20s\t%-8s\t%-s\n", "NAME", "PROTOCOL", "LISTEN")
			}
			for k, v := range entries {
<<<<<<< HEAD
				fmt.Printf("%-20s\t%-s\n", k, v.Address)
=======
				fmt.Printf("%-20s\t%-8s\t%-s\n", k, v.Protocol, v.Address)
>>>>>>> ba0155815ea4622affab23ce6558ba53e45e62a0
			}

			return nil
		},
	}
	ls.Flags().BoolVarP(&quiet, "quiet", "q", false, "Print rows without column headers")

	cmd.AddCommand(&ls)

	return cmd
}
