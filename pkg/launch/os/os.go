package os

import (
	"sync"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

// LaunchConfig is the rule for how to start up a os process.
type LaunchConfig struct {

	// Cmd is the entire command line, including options.
	Cmd string

	// SamePgID if true will make the process in the same process group as the launcher so that when the launcher
	// exists the process exits too.
	SamePgID bool
}

const (
	// DefaultExecName is the default exec name to identify this launcher in the config
	DefaultExecName = "os"
)

// NewLauncher returns a Launcher that can install and start plugins.  The OS version is simple - it translates
// plugin names as command names and uses os.Exec
func NewLauncher(n string) (*Launcher, error) {
	return &Launcher{
		name:    n,
		plugins: map[string]state{},
	}, nil
}

type state struct {
	wait <-chan error
}

// Launcher is a service that implements the launch.Exec interface for starting up os processes.
type Launcher struct {
	name    string
	plugins map[string]state
	lock    sync.Mutex
}

// Name returns the name of the launcher
func (l *Launcher) Name() string {
	return l.name
}

// Exec starts the os process. Returns a signal channel to block on optionally.
// The channel is closed as soon as an error (or nil for success completion) is written.
// The command is run in the background / asynchronously.  The returned read channel
// stops blocking as soon as the command completes (which uses shell to run the real task in
// background).
func (l *Launcher) Exec(kind string, pn plugin.Name, config *types.Any) (plugin.Name, <-chan error, error) {
	name, _ := pn.GetLookupAndType()
	launchConfig := &LaunchConfig{}
	if err := config.Decode(launchConfig); err != nil {
		return pn, nil, err
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	if s, has := l.plugins[name]; has {
		return pn, s.wait, nil
	}

	s := state{}
	l.plugins[name] = s
	s.wait = start(l, name, launchConfig.Cmd, !launchConfig.SamePgID)

	return pn, s.wait, nil
}
