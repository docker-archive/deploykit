package inproc

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "launch/inproc")

// PluginRunFunc is a function that takes the plugin lookup, a configuration blob and starts the plugin
// and returns a stoppable, running channel (for optionally blocking), and error.
type PluginRunFunc func(func() discovery.Plugins,
	*types.Any) (transport plugin.Transport, plugins map[run.PluginCode]interface{}, onStop func(), err error)

type builder struct {
	lookup  string
	run     PluginRunFunc
	options interface{}
}

var (
	builders     = map[string]builder{}
	buildersLock = sync.Mutex{}
)

// Register registers helper function with the plugin lookup name
func Register(lookup string, prf PluginRunFunc, defaultOptions interface{}) {
	buildersLock.Lock()
	defer buildersLock.Unlock()

	if _, has := builders[lookup]; has {
		panic(fmt.Sprintf("duplication of plugin name: %v", lookup))
	}
	builders[lookup] = builder{
		lookup:  lookup,
		run:     prf,
		options: defaultOptions,
	}
}

// Rules returns a list of default launch rules.  This is a set of rules required by the monitor
func Rules() []launch.Rule {
	rules := []launch.Rule{}

	for lookup, builder := range builders {

		var defaultOptions *types.Any
		if builder.options != nil {
			defaultOptions = types.AnyValueMust(builder.options)
		}

		rules = append(rules, launch.Rule{
			Plugin: plugin.Name(lookup),
			Launch: map[launch.ExecName]*types.Any{
				launch.ExecName("inproc"): defaultOptions,
			},
		})
	}

	return rules
}

// NewLauncher returns a Launcher that can install and start plugins.
// The inproc launcher will start up the plugin in process.
func NewLauncher(n string, plugins func() discovery.Plugins) (*Launcher, error) {
	return &Launcher{
		name:    n,
		running: map[string]state{},
		plugins: plugins,
	}, nil
}

type state struct {
	stoppable server.Stoppable
	running   <-chan struct{}
	wait      <-chan error
}

// Launcher is a service that implements the launch.Exec interface for starting up os processes.
type Launcher struct {
	name    string
	running map[string]state
	plugins func() discovery.Plugins
}

// Name returns the name of the launcher
func (l *Launcher) Name() string {
	return l.name
}

// Exec starts the os process. Returns a signal channel to block on optionally.
// The channel is closed as soon as an error (or nil for success completion) is written.
// The command is run in the background / asynchronously.  The returned read channel
// stops blocking as soon as the command completes.  However, the plugin is running in process.
func (l *Launcher) Exec(name string, config *types.Any) (pluginName plugin.Name, starting <-chan error, err error) {

	if s, has := l.running[name]; has {
		return pluginName, s.wait, nil
	}

	builder, has := builders[name]
	if !has {
		return pluginName, nil, fmt.Errorf("cannot start plugin %v", name)
	}

	s := state{}
	l.running[name] = s

	sc := make(chan error, 1)
	defer close(sc)

	s.wait = sc

	transport, impls, onStop, err := builder.run(l.plugins, config)
	if err != nil {
		log.Warn("error executing inproc", "plugin", name, "config", config, "err", err)
		sc <- err
		return transport.Name, s.wait, err
	}

	s.stoppable, s.running, err = run.ServeRPC(transport, onStop, impls)
	return transport.Name, s.wait, nil
}
