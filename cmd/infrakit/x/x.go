package x

import (
	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/x")

func init() {
	base.Register(Command)
}

// Command is the head of this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	experimental := &cobra.Command{
		Use:   "x",
		Short: "Experimental features",
	}

	experimental.AddCommand(
		maxlifeCommand(plugins),
		ingressCommand(plugins),
		remoteBootCommand(),
	)

	return experimental
}
