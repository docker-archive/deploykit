package x

import (
	"github.com/docker/infrakit/cmd/infrakit/base"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/x")

func init() {
	base.Register(Command)
}

// Command is the head of this module
func Command(scope scope.Scope) *cobra.Command {

	experimental := &cobra.Command{
		Use:   "x",
		Short: "Experimental features",
	}

	experimental.AddCommand(
		maxlifeCommand(scope),
		remoteBootCommand(),
		vmwscriptCommand(),
	)

	return experimental
}
