package launch

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log         = logutil.New("module", "core/launch")
	errNoConfig = errors.New("no-config")
)

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

// Merge input rule into receiver.  If the input rule's plugin doesn't match the receiver's, the receiver value
// sees no changes.
func (r Rule) Merge(o Rule) Rule {
	copy := r
	copy.Launch = map[ExecName]*types.Any{}
	for k, v := range r.Launch {
		var c types.Any
		if v != nil {
			c = *v
		}
		copy.Launch[k] = &c
	}

	if r.Plugin != o.Plugin {
		return copy
	}
	for k, v := range o.Launch {
		var c types.Any
		if v != nil {
			c = *v
		}
		copy.Launch[k] = &c
	}
	return copy
}

// Rules is a slice of rules
type Rules []Rule

func (r Rules) Len() int           { return len(r) }
func (r Rules) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r Rules) Less(i, j int) bool { return r[i].Plugin < r[j].Plugin }

// MergeRules input rules into another slice
func MergeRules(a, b []Rule) []Rule {
	out := Rules{}
	q := map[plugin.Name]Rule{}
	for _, v := range a {
		q[v.Plugin] = v
	}
	for _, r := range b {
		if found, has := q[r.Plugin]; !has {
			out = append(out, r)
		} else {
			q[r.Plugin] = found.Merge(r)
		}
	}
	for _, r := range q {
		out = append(out, r)
	}

	sort.Sort(out)
	return out
}

// Monitor runs continuously receiving requests to start a plugin.
// Monitor uses a launcher to actually start the process of the plugin.
type Monitor struct {
	execs     map[ExecName]Exec
	rules     map[plugin.Name]map[ExecName]*types.Any
	startChan <-chan StartPlugin
	inputChan chan<- StartPlugin
	stop      chan interface{}
	lock      sync.Mutex
}

// NewMonitor returns a monitor that continuously watches for input
// requests and launches the process for the plugin, if not already running.
// The configuration to use in the config is matched to the Name() of the executor (the field Exec).
func NewMonitor(execs []Exec, rules []Rule) *Monitor {
	m := map[plugin.Name]map[ExecName]*types.Any{}
	mm := map[ExecName]Exec{}

	for _, r := range rules {
		m[r.Plugin] = map[ExecName]*types.Any{}
	}

	// index by name of plugin
	for _, exec := range execs {

		n := ExecName(exec.Name())
		mm[n] = exec
		for _, r := range rules {
			if cfg, has := r.Launch[n]; has {
				m[r.Plugin][n] = cfg
			}
		}
	}
	return &Monitor{
		execs: mm,
		rules: m,
	}
}

// StartPlugin is the command to start a plugin
type StartPlugin struct {
	Plugin  plugin.Name
	Exec    ExecName
	Options *types.Any // options that can override the defaults in the rules
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
				log.Info("Plugin activation input closed. Stopping.")
				m.inputChan = nil
				return
			}

			configCopy := types.AnyBytes(nil)

			if req.Options == nil {
				// match first by full name of the form lookup/type -- 'specialization'
				properties, has := m.rules[req.Plugin][req.Exec]
				if !has {
					// match now by lookup only -- 'base class'
					alternate, _ := req.Plugin.GetLookupAndType()
					properties, has = m.rules[plugin.Name(alternate)][req.Exec]
				}
				if !has {
					log.Warn("no plugin", "plugin", req.Plugin)
					req.reportError(nil, errNoConfig)
					continue loop
				}
				if properties != nil {
					*configCopy = *properties
				}
			} else {
				*configCopy = *req.Options
			}

			exec, has := m.execs[req.Exec]
			if !has {
				req.reportError(configCopy, fmt.Errorf("no exec:%v", req.Exec))
				continue loop
			}

			log.Info("Starting plugin", "executor", exec.Name(), "plugin", req.Plugin, "exec=", req.Exec)

			block, err := exec.Exec(req.Plugin.String(), configCopy)
			if err != nil {
				req.reportError(configCopy, err)
				continue loop
			}

			log.Info("Waiting for startup", "plugin", req.Plugin, "config", configCopy.String())
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
