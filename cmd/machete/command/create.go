package command

import (
	"fmt"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

func createCmd(registry *provisioners.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "create provisioner/template",
		Short: "create a machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("I can't do that yet!")
			return nil
		},
	}
}
