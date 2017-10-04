package util

import (
	"github.com/docker/infrakit/cmd/infrakit/base"
	init_cmd "github.com/docker/infrakit/cmd/infrakit/util/init"
	"github.com/docker/infrakit/cmd/infrakit/util/mux"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

var logger = log.New("module", "cli/util")

func init() {
	base.Register(Command)
}

// Command is the head of this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	util := &cobra.Command{
		Use:   "util",
		Short: "Utilities",
	}

	util.AddCommand(
		mux.Command(plugins),
		init_cmd.Command(plugins),
		fileServerCommand(plugins),
		trackCommand(plugins),
	)

	return util
}
