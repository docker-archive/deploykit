package util

import (
	"github.com/docker/infrakit/cmd/infrakit/base"

	init_cmd "github.com/docker/infrakit/cmd/infrakit/util/init"
	"github.com/docker/infrakit/cmd/infrakit/util/mux"
	"github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
)

var logger = log.New("module", "cli/util")

func init() {
	base.Register(Command)
}

// Command is the head of this module
func Command(scp scope.Scope) *cobra.Command {

	util := &cobra.Command{
		Use:   "util",
		Short: "Utilities",
	}

	util.AddCommand(
		mux.Command(scp),
		init_cmd.Command(scp),
		fileServerCommand(scp),
		trackCommand(scp),
	)

	return util
}
