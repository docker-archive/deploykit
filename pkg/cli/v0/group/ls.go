package group

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/cli/v0/instance"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Ls returns the Ls command
func Ls(name string, services *cli.Services) *cobra.Command {

	ls := &cobra.Command{
		Use:   "ls <group ID>",
		Short: "Ls a group. Returns a list of members",
	}

	view := instance.View{}
	ls.Flags().AddFlagSet(services.OutputFlags)
	ls.Flags().AddFlagSet(view.FlagSet())

	ls.RunE = func(cmd *cobra.Command, args []string) error {

		pluginName := plugin.Name(name)
		_, gid := pluginName.GetLookupAndType()

		if gid == "" && len(args) < 1 {
			cmd.Usage()
			os.Exit(1)
		}

		if len(args) == 1 || gid == "" {
			gid = args[0]
			args = args[1:]
		}

		// get renderers first before costly rpc
		renderer, err := view.Renderer(view.DefaultMatcher(args))
		if err != nil {
			return err
		}

		groupPlugin, err := services.Scope.Group(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

		groupID := group.ID(gid)

		// TODO - here we are getting the properties back because
		// in pkg/plugin/group/scaledGroup.List we are calling the instance
		// plugin.Describe with 'true'.  We need to change the group SPI
		// to allow control of this and taking in view filter, selectors
		// to execute on the server side.
		desc, err := groupPlugin.DescribeGroup(groupID)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, desc.Instances, renderer)
	}
	return ls
}
