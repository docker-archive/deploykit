package loadbalancer

import (
	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

var log = logutil.New("module", "cli/v0/loadbalancer")

func init() {
	cli.Register(loadbalancer.InterfaceSpec,
		[]cli.CmdBuilder{
			Routes,
			Backends,
		})
}
