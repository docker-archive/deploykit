package launch

import (
	"errors"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var errNoConfig = errors.New("no-counfig")

// ExecRule encapsulates what's required to exec a plugin
type ExecRule struct {
	// Exec is the name of the exec to use to start the plugin
	Exec string
	// Properties is the properties for the executor
	Properties *Config
}

// Rule provides the instructions on starting the plugin
type Rule struct {

	// Plugin is the name of the plugin
	Plugin string

	// Launch is the rule for starting / launching the plugin.
	Launch ExecRule
}

// Monitor runs continuously receiving requests to start a plugin.
// Monitor uses a launcher to actually start the process of the plugin.
type Monitor struct {
	exec      Exec
	rules     map[string]Rule
	startChan <-chan StartPlugin
	inputChan chan<- StartPlugin
	stop      chan interface{}
	lock      sync.Mutex
}

// NewMonitor returns a monitor that continuously watches for input
// requests and launches the process for the plugin, if not already running.
func NewMonitor(l Exec, rules []Rule) *Monitor {
	m := map[string]Rule{}
	// index by name of plugin
	for _, r := range rules {
		if r.Launch.Exec == l.Name() {
			m[r.Plugin] = r
		}
	}
	return &Monitor{
		exec:  l,
		rules: m,
	}
}

// StartPlugin is the command to start a plugin
type StartPlugin struct {
	Plugin  string
	Started func(*Config)
	Error   func(*Config, error)
}

func (s StartPlugin) reportError(config *Config, e error) {
	if s.Error != nil {
		go s.Error(config, e)
	}
}

func (s StartPlugin) reportSuccess(config *Config) {
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

			r, has := m.rules[req.Plugin]
			if !has {
				log.Warningln("no plugin:", req)
				req.reportError(r.Launch.Properties, errNoConfig)
				continue loop
			}

			configCopy := &Config{}
			if r.Launch.Properties != nil {
				*configCopy = *r.Launch.Properties
			}

			block, err := m.exec.Exec(r.Plugin, configCopy)
			if err != nil {
				req.reportError(configCopy, err)
				continue loop
			}

			log.Infoln("Waiting for", r.Plugin, "to start:", configCopy.String())
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
