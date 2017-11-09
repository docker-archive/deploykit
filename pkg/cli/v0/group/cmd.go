package group

import (
	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/v1/group")

func init() {
	cli.Register(group.InterfaceSpec,
		[]cli.CmdBuilder{
			// Group,
			Ls,
			Inspect,
			Describe,
			Commit,
			Free,
			Destroy,
			Scale,
			DestroyInstances,
		})
}

// Group returns the group command
func Group(name string, services *cli.Services) *cobra.Command {
	group := &cobra.Command{
		Use:   "group",
		Short: "Commands to access the Group SPI",
	}

	group.AddCommand(
		Ls(name, services),
		Inspect(name, services),
		Describe(name, services),
		Commit(name, services),
		Free(name, services),
		Destroy(name, services),
		Scale(name, services),
		DestroyInstances(name, services),
	)

	return group
}
