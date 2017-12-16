package manager

import (
	"io/ioutil"
	sys_os "os"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/launch/inproc"
	"github.com/docker/infrakit/pkg/launch/os"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log        = logutil.New("module", "run/manager")
	debugLoopV = logutil.V(1000)
)

// ManagePlugins returns a manager that can manage the start up and stopping of plugins.
func ManagePlugins(rules []launch.Rule, scope scope.Scope, mustAll bool, scanInterval time.Duration) (*Manager, error) {

	m := &Manager{
		scope:        scope,
		mustAll:      mustAll,
		scanInterval: scanInterval,
		running:      []string{},
	}
	return m, m.Start(rules)
}

// Manager manages the plugins startup, stop, etc.
type Manager struct {
	// mustAll panics if set to true and any plugins fails to start
	mustAll bool
	// scanInterval is the interval for checking the plugin discovery
	scanInterval time.Duration

	scope scope.Scope

	rules       []launch.Rule
	monitor     *launch.Monitor
	startPlugin chan<- launch.StartPlugin
	wgStartAll  sync.WaitGroup
	started     chan plugin.Name
	lock        sync.RWMutex

	running []string // lookups of those started
}

// Rules returns a list of plugins that can be launched via this manager
func (m *Manager) Rules() []launch.Rule {
	return m.rules
}

// TerminateRunning terminates those that have been started.
func (m *Manager) TerminateRunning() error {
	return m.Terminate(m.running)
}

// Terminate stops the plugins.  Note this is accomplished by sending a signal TERM to the
// process found at the lookup.pid file.  For inproc plugins, this will effectively kill
// all the plugins that run in that process.
// TODO - selectively terminate inproc plugins without taking down the process.
func (m *Manager) Terminate(lookup []string) error {
	allPlugins, err := m.scope.Plugins().List()
	if err != nil {
		return err
	}
	for _, n := range lookup {

		p, has := allPlugins[n]
		if !has {
			continue
		}

		pidFile := n + ".pid"
		if p.Protocol == "unix" {
			pidFile = p.Address + ".pid"
		} else {
			pidFile = path.Join(local.Dir(), pidFile)
		}

		buff, err := ioutil.ReadFile(pidFile)
		if err != nil {
			log.Warn("Cannot read PID file", "name", n, "pid", pidFile)
			continue
		}

		pid, err := strconv.Atoi(string(buff))
		if err != nil {
			log.Warn("Cannot determine PID", "name", n, "pid", pidFile)
			continue
		}

		process, err := sys_os.FindProcess(pid)
		if err != nil {
			log.Warn("Error finding process of plugin", "name", n)
			continue
		}

		log.Info("Stopping", "name", n, "pid", pid)
		if err := process.Signal(syscall.SIGTERM); err == nil {
			process.Wait()
			log.Info("Process exited", "name", n)
		}

	}
	return nil
}

// TerminateAll terminates all the plugins.
func (m *Manager) TerminateAll() error {
	allPlugins, err := m.scope.Plugins().List()
	if err != nil {
		return err
	}
	names := []string{}
	for n := range allPlugins {
		names = append(names, n)
	}
	return m.Terminate(names)
}

// Start starts the manager
func (m *Manager) Start(rules []launch.Rule) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.started != nil {
		return nil
	}

	// launch plugin via os process
	osExec, err := os.NewLauncher(os.DefaultExecName)
	if err != nil {
		return err
	}
	// launch inprocess plugins
	inprocExec, err := inproc.NewLauncher(inproc.DefaultExecName, m.scope)
	if err != nil {
		return err
	}

	m.rules = launch.MergeRules(inproc.Rules(), rules)
	m.monitor = launch.NewMonitor([]launch.Exec{
		osExec,
		inprocExec,
	}, m.rules)

	// start the monitor
	startPlugin, err := m.monitor.Start()
	if err != nil {
		return err
	}

	m.startPlugin = startPlugin
	m.started = make(chan plugin.Name, 100)
	if m.scanInterval == 0 {
		m.scanInterval = 5 * time.Second
	}
	return nil
}

func stringFrom(a *types.Any) string {
	if a != nil {
		return a.String()
	}
	return "<nil>"
}

// Launch launches the plugin
func (m *Manager) Launch(exec string, key string, name plugin.Name, options *types.Any) error {
	m.lock.RLock()
	defer m.lock.RUnlock()

	// check that the plugin is not currently running
	running, err := m.scope.Plugins().List()
	if err != nil {
		return err
	}

	lookup, _ := name.GetLookupAndType()
	if countMatches([]string{lookup}, running) > 0 {
		log.Debug("already running", "lookup", lookup, "name", name)
		m.started <- name
		return nil
	}
	m.wgStartAll.Add(1)

	log.Debug("starting", "key", key, "name", name, "exec", exec, "options", options)
	if m.startPlugin == nil {
		log.Info("monitor not running anymore")
		return nil
	}

	m.startPlugin <- launch.StartPlugin{
		Key:     key,
		Name:    name,
		Exec:    launch.ExecName(exec),
		Options: options,
		Started: func(key string, n plugin.Name, config *types.Any) {
			m.started <- n
			m.wgStartAll.Done()
			log.Debug("started", "key", key, "name", name, "exec", exec, "options", options)
		},
		Error: func(key string, n plugin.Name, config *types.Any, err error) {
			log.Error("error starting", "key", key, "name", name, "exec", exec, "options", options)
			if m.mustAll {
				log.Crit("Terminating due to error starting plugin", "err", err,
					"key", key, "name", n, "config", stringFrom(config))
				panic(err)
			}
			m.wgStartAll.Done()
		},
	}
	return nil
}

// WaitForAllShutdown blocks until all the plugins stopped.
func (m *Manager) WaitForAllShutdown() {
	targets := []string{}
	seen := map[string]struct{}{}
	checkNow := time.Tick(m.scanInterval)

	for {
		select {
		case target := <-m.started:

			lookup, _ := target.GetLookupAndType()

			m.running = append(m.running, lookup)

			if _, has := seen[lookup]; !has {
				log.Debug("Start watching", "lookup", lookup)
				targets = append(targets, lookup)
				seen[lookup] = struct{}{}
			}

		case <-checkNow:
			log.Debug("Checking on targets", "targets", targets, "V", debugLoopV)
			if m, err := m.scope.Plugins().List(); err == nil {
				if countMatches(targets, m) == 0 {
					log.Info("Scan found plugins not running now", "plugins", targets)
					return
				}
			}
		}
	}
}

// counts the number of matches by name
func countMatches(list []string, found map[string]*plugin.Endpoint) int {
	c := 0
	for _, l := range list {
		if ep, has := found[l]; has {
			// testing it with a handshaker
			hs, err := client.NewHandshaker(ep.Address)
			if err != nil {
				log.Error("Plugin not responding", "lookup", l, "endpoint", ep, "err", err)
				continue
			}
			objects, err := hs.Hello()
			if err != nil {
				log.Error("Bad handshake. Is this plugin running?", "lookup", l, "endpoint", ep, "err", err)
				continue
			}
			log.Debug("Scan found", "lookup", l, "endpoint", ep, "V", debugLoopV, "implements", objects)
			c++
		}
	}
	return c
}

// WaitStarting blocks until a current batch of plugins completed starting up.
func (m *Manager) WaitStarting() {
	m.wgStartAll.Wait()
}

// Stop stops the manager
func (m *Manager) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.monitor.Stop()
	m.startPlugin = nil
	log.Debug("Stopped plugin manager")
}
