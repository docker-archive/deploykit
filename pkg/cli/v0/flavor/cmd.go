package flavor

import (
	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/flavor"
)

var log = logutil.New("module", "cli/v1/flavor")

func init() {
	cli.Register(flavor.InterfaceSpec,
		[]cli.CmdBuilder{
			Validate,
			Prepare,
			Healthy,
		})
}
