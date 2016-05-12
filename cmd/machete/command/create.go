package command

import (
	"fmt"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

type create struct {
	output         console.Console
	machineCreator libmachete.MachineCreator
}

func (c *create) run(args []string) error {
	if len(args) != 2 {
		return UsageError
	}

	// TODO(wfarner): Generalize this once we have plumbing for additional parameters.
	createEvents, err := c.machineCreator.Create(
		args[0],
		map[string]string{"REGION": "us-west-2"},
		args[1],
		[]byte{})
	if err != nil {
		return fmt.Errorf("Machine creation could not start: %s", err)
	}

	for event := range createEvents {
		c.output.Println(event)
	}

	return nil
}

func createCmd(
	output console.Console,
	registry *provisioners.Registry,
	templates libmachete.TemplateLoader) *cobra.Command {

	cmd := create{
		output:         output,
		machineCreator: libmachete.NewCreator(registry, templates)}

	return &cobra.Command{
		Use:   "create provisioner template",
		Short: "create a machine",
		RunE: func(_ *cobra.Command, args []string) error {
			return cmd.run(args)
		},
	}
}
