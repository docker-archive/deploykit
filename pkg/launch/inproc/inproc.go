package inproc

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/launch"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "launch/inproc")

const (
	// ExecName is the name to use in the launch rule configs
	ExecName = "inproc"
)

// PluginRunFunc is a function that takes the plugin lookup, a configuration blob and starts the plugin
// and returns a stoppable, running channel (for optionally blocking), and error.
type PluginRunFunc func(scope.Scope, plugin.Name, *types.Any) (transport plugin.Transport,
	plugins map[run.PluginCode]interface{}, onStop func(), err error)

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

// Rule is the rule for starting an inproc plugin.
type Rule struct {

	// Kind is the canonical name that are defined for each package under pkg/run/v[0-9]+.
	// Kind is an organization of multiple plugin's. For example, there is a kubernetes Kind and this
	// would correspond to pkg/types/Spec.Kind.  This is used to identify the subsystem to start (e.g. kubernetes).
	// However, it is possible to have multiple instances of objects in a same Kind.  For example, for aws Kind,
	// it's possible to have two instance plugins, one called us-west-1a and one us-west-2a. So the kind is aws,
	// but the lookup name for discovery (which plugin.Name is used), would be us-west-1a.sock and us-west-1b.sock,
	// and each endpoint would have multiple objects (e.g. us-west-1a/ec2-instance and us-west-1b/ec2-instance).
	Kind    string
	Options *types.Any
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
			Key: lookup,
			Launch: map[launch.ExecName]*types.Any{
				launch.ExecName("inproc"): types.AnyValueMust(
					Rule{
						Kind:    lookup,
						Options: defaultOptions,
					},
				),
			},
		})
	}

	return rules
}

const (
	// DefaultExecName is the default exec name to identify this launcher in the config
	DefaultExecName = "inproc"
)

// NewLauncher returns a Launcher that can install and start plugins.
// The inproc launcher will start up the plugin in process.
func NewLauncher(n string, scope scope.Scope) (*Launcher, error) {
	return &Launcher{
		name:    n,
		running: map[string]state{},
		scope:   scope,
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
	scope   scope.Scope
}

// Name returns the name of the launcher
func (l *Launcher) Name() string {
	return l.name
}

// Exec starts the os process. Returns a signal channel to block on optionally.
// The channel is closed as soon as an error (or nil for success completion) is written.
// The command is run in the background / asynchronously.  The returned read channel
// stops blocking as soon as the command completes.  However, the plugin is running in process.
func (l *Launcher) Exec(key string, pn plugin.Name,
	config *types.Any) (pluginName plugin.Name, starting <-chan error, err error) {

	name, _ := pn.GetLookupAndType()

	log.Debug("exec", "key", key, "pn", pn, "config", config)

	if s, has := l.running[name]; has {
		return pluginName, s.wait, nil
	}

	inprocRule := Rule{}
	err = config.Decode(&inprocRule)
	if err != nil {
		return
	}

	log.Debug("parsed rule", "key", key, "pn", pn, "config", config, "rule", inprocRule)

	builder, has := builders[inprocRule.Kind]
	if !has {
		return pluginName, nil, fmt.Errorf("cannot start plugin of kind %v", inprocRule.Kind)
	}

	s := state{}
	l.running[name] = s

	sc := make(chan error, 1)
	defer close(sc)

	s.wait = sc

	log.Debug("about to run", "key", key, "name", name, "config", config, "options", inprocRule.Options)

	transport, impls, onStop, err := builder.run(l.scope, plugin.Name(name), inprocRule.Options)
	if err != nil {
		log.Warn("error executing inproc", "plugin", name, "config", inprocRule.Options, "err", err)
		sc <- err
		return transport.Name, s.wait, err
	}

	s.stoppable, s.running, err = run.ServeRPC(transport, onStop, impls)
	return transport.Name, s.wait, nil
}
