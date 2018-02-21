package instance

import (
	"os"
	"sort"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

// Describe returns the describe command
func Describe(name string, services *cli.Services) *cobra.Command {
	describe := &cobra.Command{
		Use:   "describe",
		Short: "Describe all managed instances across all groups, subject to filter",
	}

	view := View{}
	describe.Flags().AddFlagSet(services.OutputFlags)
	describe.Flags().AddFlagSet(view.FlagSet())

	describe.RunE = func(cmd *cobra.Command, args []string) error {
		// get renderers first before costly rpc
		renderer, err := view.Renderer(view.DefaultMatcher(args))
		if err != nil {
			return err
		}

		instancePlugin, err := services.Scope.Instance(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(instancePlugin, "instance plugin not found", "name", name)

		desc, err := instancePlugin.DescribeInstances(view.TagFilter(), view.ShowProperties())
		if err != nil {
			return err
		}

		instances := instance.Descriptions(desc)
		sort.Sort(instances)

		return services.Output(os.Stdout, []instance.Description(instances), renderer)
	}
	return describe
}
