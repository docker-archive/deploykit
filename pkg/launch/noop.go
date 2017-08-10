package launch

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

type noOp int

// NewNoOpExec doesn't actually launch the plugins.  It's a stub with no op and relies on manual plugin starts.
func NewNoOpExec() Exec {
	return noOp(0)
}

// Name returns the name of the exec
func (n noOp) Name() string {
	return "noop"
}

// Launch starts the plugin given the name
func (n noOp) Exec(kind string, name plugin.Name, config *types.Any) (plugin.Name, <-chan error, error) {
	log.Info("NO-OP Exec: not automatically starting plugin", "plugin", name, "args", config)

	starting := make(chan error)
	close(starting) // channel won't block
	return name, starting, nil
}
