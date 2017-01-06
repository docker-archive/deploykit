package launch

import (
	log "github.com/Sirupsen/logrus"
)

type noOp int

// NewNoOpLauncher doesn't actually launch the plugins.  It's a stub with no op and relies on manual plugin starts.
func NewNoOpLauncher() Launcher {
	return noOp(0)
}

// Launch starts the plugin given the name
func (n noOp) Launch(name, cmd string, args ...string) (<-chan error, error) {
	log.Infoln("NO-OP Launcher: not automatically starting plugin", name, "args=", args)

	starting := make(chan error)
	close(starting) // channel won't block
	return starting, nil
}
