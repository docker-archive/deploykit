package main

import (
	"errors"
	"fmt"

	"github.com/docker/libmachete/discovery"
	"github.com/spf13/cobra"
)

func pluginCommand(pluginDir func() *discovery.Dir) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
		RunE: func(c *cobra.Command, args []string) error {

			if pluginDir() == nil {
				return errors.New("can't lookup plugins")
			}

			if len(args) != 1 {
				return errors.New("need one more arg")
			}

			switch args[0] {

			case "ls":
				entries, err := pluginDir().List()
				if err != nil {
					return err
				}

				fmt.Println("Plugins:")
				fmt.Printf("%-10s\t%-s\n", "name", "URL")
				for k, v := range entries {
					fmt.Printf("%-10s\t%-s\n", k, v.String())
				}

			default:
			}

			return nil
		},
	}
	return cmd
}
