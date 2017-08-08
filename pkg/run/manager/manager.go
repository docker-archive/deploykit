package manager

import (
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/launch/inproc"
	"github.com/docker/infrakit/pkg/launch/os"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "run/manager")

// ManagePlugins returns a manager that can manage the start up and stopping of plugins.
func ManagePlugins(rules []launch.Rule,
	plugins func() discovery.Plugins, mustAll bool, scanInterval time.Duration) (*Manager, error) {

	m := &Manager{
		plugins:      plugins,
		mustAll:      mustAll,
		scanInterval: scanInterval,
	}
	return m, m.Start(rules)
}

// Manager manages the plugins startup, stop, etc.
type Manager struct {
	// mustAll panics if set to true and any plugins fails to start
	mustAll bool
	// scanInterval is the interval for checking the plugin discovery
	scanInterval time.Duration
	// Plugins is a function that returns the plugins discovered.
	plugins func() discovery.Plugins

	rules       []launch.Rule
	monitor     *launch.Monitor
	startPlugin chan<- launch.StartPlugin
	wgStartAll  sync.WaitGroup
	started     chan plugin.Name
	startedAll  chan struct{}
	lock        sync.Mutex
}

// Start starts the manager
func (m *Manager) Start(rules []launch.Rule) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.started != nil {
		return nil
	}

	m.rules = rules
	// launch plugin via os process
	osExec, err := os.NewLauncher("os")
	if err != nil {
		return err
	}
	// launch docker run, implemented by the same os executor. We just search for a different key (docker-run)
	dockerExec, err := os.NewLauncher("docker-run")
	if err != nil {
		return err
	}
	// launch inprocess plugins
	inprocExec, err := inproc.NewLauncher("inproc", m.plugins)
	if err != nil {
		return err
	}

	m.monitor = launch.NewMonitor([]launch.Exec{
		osExec,
		dockerExec,
		inprocExec,
	}, launch.MergeRules(inproc.Rules(), rules))

	// start the monitor
	startPlugin, err := m.monitor.Start()
	if err != nil {
		return err
	}

	m.startPlugin = startPlugin
	m.started = make(chan plugin.Name, 100)
	m.startedAll = make(chan struct{})
	if m.scanInterval == 0 {
		m.scanInterval = 5 * time.Second
	}
	go func() {
		for {
			m.wgStartAll.Wait()
			if m.startedAll != nil {
				close(m.startedAll)
				m.startedAll = nil
			}
		}
	}()
	return nil
}

// Launch launches the plugin
func (m Manager) Launch(exec string, name plugin.Name, options *types.Any) error {

	// check that the plugin is not currently running
	running, err := m.plugins().List()
	if err != nil {
		return err
	}

	lookup, _ := name.GetLookupAndType()
	if countMatches([]string{lookup}, running) > 0 {
		m.started <- name
		return nil
	}

	m.startPlugin <- launch.StartPlugin{
		Plugin:  name,
		Exec:    launch.ExecName(exec),
		Options: options,
		Started: func(n plugin.Name, config *types.Any) {
			m.started <- n
			m.wgStartAll.Done()
		},
		Error: func(n plugin.Name, config *types.Any, err error) {
			if m.mustAll {
				panic(err)
			}
			m.wgStartAll.Done()
		},
	}
	m.wgStartAll.Add(1)
	return nil
}

// Wait blocks until all the plugins stopped.
func (m Manager) WaitForAllShutdown() {
	targets := []string{}
	checkNow := time.Tick(m.scanInterval)

	for {
		select {
		case target := <-m.started:
			lookup, _ := target.GetLookupAndType()
			log.Debug("Start watching", "lookup", lookup)
			targets = append(targets, lookup)

		case <-checkNow:
			log.Debug("Checking on targets", "targets", targets)
			if m, err := m.plugins().List(); err == nil {
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
		if _, has := found[l]; has {
			log.Debug("Scan found", "lookup", l)
			c++
		}
	}
	return c
}

// WaitStaring blocks until a current batch of plugins completed starting up.
func (m Manager) WaitStarting() {
	if m.startedAll != nil {
		<-m.startedAll
	}
}

// Stop stops the manager
func (m Manager) Stop() {
	m.monitor.Stop()
	close(m.started)
	if m.startedAll != nil {
		close(m.startedAll)
	}
	log.Info("Stopped plugin manager")
}
