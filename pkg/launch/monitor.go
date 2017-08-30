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

	// Key is a string identifying which rule to use.  This can be used in the command line, e.g. plugin start.
	Key string

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

	if r.Key != o.Key {
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
func (r Rules) Less(i, j int) bool { return r[i].Key < r[j].Key }

// MergeRules input rules into another slice
func MergeRules(a, b []Rule) []Rule {
	out := Rules{}
	q := map[string]Rule{}
	for _, v := range a {
		q[v.Key] = v
	}
	for _, r := range b {
		if found, has := q[r.Key]; !has {
			out = append(out, r)
		} else {
			q[r.Key] = found.Merge(r)
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
	rules     map[string]map[ExecName]*types.Any
	startChan <-chan StartPlugin
	inputChan chan<- StartPlugin
	stop      chan interface{}
	lock      sync.Mutex
}

// NewMonitor returns a monitor that continuously watches for input
// requests and launches the process for the plugin, if not already running.
// The configuration to use in the config is matched to the Name() of the executor (the field Exec).
func NewMonitor(execs []Exec, rules []Rule) *Monitor {
	m := map[string]map[ExecName]*types.Any{}
	mm := map[ExecName]Exec{}

	for _, r := range rules {
		m[r.Key] = map[ExecName]*types.Any{}
	}

	// index by name of plugin
	for _, exec := range execs {

		n := ExecName(exec.Name())
		mm[n] = exec
		for _, r := range rules {
			if cfg, has := r.Launch[n]; has {
				m[r.Key][n] = cfg
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
	Key     string
	Name    plugin.Name
	Exec    ExecName
	Options *types.Any // options that can override the defaults in the rules
	Started func(kind string, name plugin.Name, options *types.Any)
	Error   func(kind string, name plugin.Name, options *types.Any, err error)
}

func (s StartPlugin) reportError(kind string, n plugin.Name, config *types.Any, e error) {
	if s.Error != nil {
		go s.Error(kind, n, config, e)
	}
}

func (s StartPlugin) reportSuccess(kind string, n plugin.Name, config *types.Any) {
	if s.Started != nil {
		go s.Started(kind, n, config)
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
				m.inputChan = nil
				log.Debug("Plugin activation input closed. Stopping.")
				return
			}

			configCopy := types.AnyBytes(nil)

			if req.Options == nil {
				// match first by full name of the form lookup/type -- 'specialization'
				properties, has := m.rules[req.Key][req.Exec]
				if !has {
					log.Warn("no plugin kind defined", "key", req.Key)
					req.reportError(req.Key, plugin.Name(""), nil, errNoConfig)
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
				req.reportError(req.Key, plugin.Name(""), configCopy, fmt.Errorf("no exec:%v", req.Exec))
				continue loop
			}

			log.Info("Starting plugin", "executor", exec.Name(), "key", req.Key, "name", req.Name, "exec", req.Exec)

			name, block, err := exec.Exec(req.Key, req.Name, configCopy)
			if err != nil {
				log.Warn("error starting plugin", "err", err, "config", configCopy,
					"key", req.Key, "name", req.Name, "as", name)
				req.reportError(req.Key, req.Name, configCopy, err)
				continue loop
			}

			log.Info("Waiting for startup", "key", req.Key, "name", req.Name,
				"config", configCopy.String(), "as", name)
			err = <-block
			if err != nil {
				log.Warn("error startup", "err", err, "config", configCopy, "key", req.Key, "name", req.Name)
				req.reportError(req.Key, name, configCopy, err)
				continue loop
			}

			req.reportSuccess(req.Key, name, configCopy)
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
