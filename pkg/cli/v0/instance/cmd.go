package instance

import (
	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
)

var log = logutil.New("module", "cli/v1/instance")

func init() {
	cli.Register(instance.InterfaceSpec,
		[]cli.CmdBuilder{
			Validate,
			Provision,
			Describe,
			Destroy,
		})
}
