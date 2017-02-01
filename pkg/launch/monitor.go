package launch

import (
	"errors"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

var errNoConfig = errors.New("no-config")

// ExecName is the name of the executor to use (e.g. 'os', 'docker-run', etc.). It's found in the config.
type ExecName string

// Rule provides the instructions on starting the plugin
type Rule struct {

	// Plugin is the name of the plugin
	Plugin plugin.Name

	// Launch is the rule for starting / launching the plugin. It's a dictionary with the key being
	// the name of the executor and the value being the properties used by that executor.
	Launch map[ExecName]*types.Any
}

// Monitor runs continuously receiving requests to start a plugin.
// Monitor uses a launcher to actually start the process of the plugin.
type Monitor struct {
	exec      Exec
	rules     map[plugin.Name]*types.Any
	startChan <-chan StartPlugin
	inputChan chan<- StartPlugin
	stop      chan interface{}
	lock      sync.Mutex
}

// NewMonitor returns a monitor that continuously watches for input
// requests and launches the process for the plugin, if not already running.
// The configuration to use in the config is matched to the Name() of the executor (the field Exec).
func NewMonitor(l Exec, rules []Rule) *Monitor {
	m := map[plugin.Name]*types.Any{}
	// index by name of plugin
	for _, r := range rules {
		if cfg, has := r.Launch[ExecName(l.Name())]; has {
			m[r.Plugin] = cfg
		}
	}
	return &Monitor{
		exec:  l,
		rules: m,
	}
}

// StartPlugin is the command to start a plugin
type StartPlugin struct {
	Plugin  plugin.Name
	Started func(*types.Any)
	Error   func(*types.Any, error)
}

func (s StartPlugin) reportError(config *types.Any, e error) {
	if s.Error != nil {
		go s.Error(config, e)
	}
}

func (s StartPlugin) reportSuccess(config *types.Any) {
	if s.Started != nil {
		go s.Started(config)
	}
}

// Start starts the monitor and returns a channel for sending
// requests to launch plugins.  Closing the channel effectively stops
// the monitor loop.
func (m *Monitor) Start() (chan<- StartPlugin, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.startChan != nil {
		return m.inputChan, nil
	}

	ch := make(chan StartPlugin)
	m.startChan = ch
	m.inputChan = ch

	go func() {
	loop:
		for {
			req, open := <-m.startChan
			if !open {
				log.Infoln("Plugin activation input closed. Stopping.")
				m.inputChan = nil
				return
			}

			// match first by full name of the form lookup/type -- 'specialization'
			properties, has := m.rules[req.Plugin]
			if !has {
				// match now by lookup only -- 'base class'
				alternate, _ := req.Plugin.GetLookupAndType()
				properties, has = m.rules[plugin.Name(alternate)]
			}
			if !has {
				log.Warningln("no plugin:", req)
				req.reportError(nil, errNoConfig)
				continue loop
			}

			configCopy := types.AnyBytes(nil)
			if properties != nil {
				*configCopy = *properties
			}

			block, err := m.exec.Exec(req.Plugin.String(), configCopy)
			if err != nil {
				req.reportError(configCopy, err)
				continue loop
			}

			log.Infoln("Waiting for", req.Plugin, "to start:", configCopy.String())
			err = <-block
			if err != nil {
				req.reportError(configCopy, err)
				continue loop
			}

			req.reportSuccess(configCopy)
		}
	}()

	return m.inputChan, nil
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.inputChan != nil {
		close(m.inputChan)
	}
}
