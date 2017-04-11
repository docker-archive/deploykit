package application

import (
	"fmt"
	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	application_rpc "github.com/docker/infrakit/pkg/rpc/application"
	//	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/application"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/event")

//OPERATIONS code of update command
var OPERATIONS = map[int]string{1: "Add", 2: "Delete", 3: "Update", 4: "Read"}

func init() {
	base.Register(Command)
}

// Command is the entry point of the module
func Command(plugins func() discovery.Plugins) *cobra.Command {
	var applicationPlugin application.Plugin

	cmd := &cobra.Command{
		Use:   "application",
		Short: "Access application plugins",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
			return err
		}

		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		p, err := application_rpc.NewClient(plugin.Name(*name), endpoint.Address)
		if err != nil {
			return err
		}
		applicationPlugin = p

		cli.MustNotNil(applicationPlugin, "application plugin not found", "name", *name)
		return nil
	}

	operation := 3
	resource := ""
	value := ""
	update := &cobra.Command{
		Use:   "update",
		Short: "Update application's resouce",
		RunE: func(c *cobra.Command, args []string) error {
			fmt.Printf("send update message plugin=%v, op=%v, resource=%v, value=%v.\n", args, OPERATIONS[operation], resource, value)
			err := applicationPlugin.Update(
				&application.Message{
					Op:       application.Operation(operation),
					Resource: resource,
					Data:     types.AnyString(value),
				},
			)
			if err != nil {
				return err
			}

			return nil
		},
	}
	update.Flags().IntVar(&operation, "op", operation, "update operation 1: Add, 2: Delete, 3: Update, 4: Read(default)")
	update.Flags().StringVar(&resource, "resource", resource, "target resource")
	update.Flags().StringVar(&value, "value", value, "update value")

	cmd.AddCommand(update)

	return cmd
}
