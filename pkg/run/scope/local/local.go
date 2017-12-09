package local

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/manager"
	"github.com/docker/infrakit/pkg/run/scope"
	group_kind "github.com/docker/infrakit/pkg/run/v0/group"
	manager_kind "github.com/docker/infrakit/pkg/run/v0/manager"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log    = logutil.New("module", "run/scope/local")
	debugV = logutil.V(300)
)

// StartPlugin is a specification of what plugin to run and what socket name
// to use, etc.
// The format is kind[:{plugin_name}][={os|inproc}]
type StartPlugin string

// FromAddressable returns a StartPlugin encoded string
func FromAddressable(addr plugin.Addressable) StartPlugin {
	return StartPlugin(fmt.Sprintf("%v:%v", addr.Kind(), addr.Plugin().Lookup()))
}

// Parse parses the specification into parts that the manager can use to launch plugins
func (arg StartPlugin) Parse() (execName string, kind string, name plugin.Name, err error) {
	p := strings.Split(string(arg), "=")
	execName = "inproc" // default is to use inprocess goroutine for running plugins
	if len(p) > 1 {
		execName = p[1]
	}

	// the format is kind[:{plugin_name}][={os|inproc}]
	pp := strings.Split(p[0], ":")
	kind = pp[0]
	name = plugin.Name(kind)

	switch kind {
	case manager_kind.Kind:
		name = plugin.Name(manager_kind.LookupName)
	case group_kind.Kind:
		name = plugin.Name(group_kind.LookupName)
	}

	// customized by user as override
	if len(pp) > 1 {
		name = plugin.Name(pp[1])
	}

	if kind == "" || execName == "" {
		err = fmt.Errorf("invalid launch spec: %v", arg)
	}
	return
}

// Options are tuning parameters for executing a task in context of
// a set of plugins that are started as required.
type Options struct {
	// StartWait is how long to wait to make sure all plugins are up
	StartWait types.Duration
	// StopWait is how long to wait to make sure all plugins are shut down
	StopWait types.Duration
}

// Execute runs a unit of work with the specified list of plugins
// running.
func Execute(plugins func() discovery.Plugins,
	pluginManager *manager.Manager,
	starts func() ([]StartPlugin, error),
	do scope.Work, options Options) error {

	pluginsToStart, err := starts()
	if err != nil {
		return err
	}

	// first start up the plugins
	for _, plugin := range pluginsToStart {
		execName, kind, name, err := plugin.Parse()
		if err != nil {
			return err
		}
		err = pluginManager.Launch(execName, kind, name, nil)
		if err != nil {
			log.Warn("failed to launch", "exec", execName, "kind", kind, "name", name)
			return err
		}
	}

	defer func() {
		<-time.After(options.StopWait.Duration())
		pluginManager.TerminateAll()
		pluginManager.WaitForAllShutdown()
		pluginManager.Stop()
	}()

	pluginManager.WaitStarting()
	<-time.After(options.StartWait.Duration())

	log.Debug("Executing work in scope", "V", debugV)
	err = do(scope.DefaultScope(plugins)) // full access
	if err != nil {
		log.Error("error processing in scope", "err", err)
	}
	return err
}
