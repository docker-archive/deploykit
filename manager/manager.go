package manager

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/discovery"
	"github.com/docker/infrakit/leader"
	rpc "github.com/docker/infrakit/rpc/group"
	"github.com/docker/infrakit/spi/group"
	"github.com/docker/infrakit/store"
)

// Backend is the admin / server interface
type Backend interface {
	group.Plugin

	Start() (<-chan struct{}, error)
	Stop()
}

// manager is the controller of all the plugins.  It is able to process multiple inputs
// such as leadership changes and configuration changes and perform the necessary actions
// to activate / deactivate plugins
type manager struct {
	group.Plugin

	plugins  discovery.Plugins
	leader   leader.Detector
	snapshot store.Snapshot
	isLeader bool
	lock     sync.Mutex
	stop     chan struct{}
	running  chan struct{}

	backendName string
	backendOps  chan<- backendOp
}

type backendOp struct {
	name      string
	operation func() error
	err       chan<- error
}

// NewManager returns the manager which depends on other services to coordinate and manage
// the plugins in order to ensure the infrastructure state matches the user's spec.
func NewManager(plugins discovery.Plugins, leader leader.Detector, snapshot store.Snapshot,
	backendName string) (Backend, error) {

	m := &manager{
		plugins:  plugins,
		leader:   leader,
		snapshot: snapshot,
	}

	gp, err := m.proxyForGroupPlugin(backendName)
	if err != nil {
		return nil, err
	}

	m.Plugin = gp
	return m, nil
}

// return true only if the current call caused an allocation of the running channel.
func (m *manager) initRunning() bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.running != nil {
		m.running = make(chan struct{})
		return true
	}
	return false
}

// Start starts the manager.  It does not block. Instead read from the returned channel to block.
func (m *manager) Start() (<-chan struct{}, error) {

	// initRunning guarantees that the m.running will be initialized the first time it's
	// called.  If another call of Start is made after the first, don't do anything just return the references.
	if !m.initRunning() {
		return m.running, nil
	}

	leaderChan, err := m.leader.Start()
	if err != nil {
		return nil, err
	}

	m.stop = make(chan struct{})
	notify := make(chan bool)
	stopWorkQueue := make(chan struct{})

	// proxied backend needs to have its operations serialized with respect to leadership calls, etc.
	backendOps := make(chan backendOp)
	m.backendOps = backendOps

	// This goroutine here serializes work so that we don't have concurrent commits or unwatches / updates / etc.
	go func() {

		for {
			select {

			case op := <-backendOps:
				log.Debugln("Backend operation:", op)
				if m.isLeader {
					op.err <- op.operation()
				} else {
					op.err <- fmt.Errorf("not-a-leader")
				}

			case <-stopWorkQueue:

				log.Infoln("Stopping work queue.")
				close(m.running)
				log.Infoln("Manager stopped.")
				return

			case leader := <-notify:

				// This channel has data only when there's been a leadership change.

				log.Debugln("leader:", leader)
				if leader {
					m.onAssumeLeadership()
				} else {
					m.onLostLeadership()
				}
			}

		}
	}()

	// Goroutine for handling all inbound leadership and control events.
	go func() {

		for {
			select {

			case <-m.stop:

				log.Infoln("Stopping..")
				close(stopWorkQueue)
				close(notify)
				return

			case evt := <-leaderChan:
				// This here handles possible duplicated events about leadership and fires only when there
				// is a change.

				m.lock.Lock()

				current := m.isLeader

				if evt.Status == leader.Unknown {
					log.Warningln("Leadership status is uncertain:", evt.Error)

					// if we are currently the leader then there's a possibility of split brain depending on
					// the robustness of the leader election process.
					// It's better to be conservative and not assume we can still be a leader...  just downgrade
					// because the worst case is we stopped watching (and not have two masters running wild).

					if m.isLeader {
						m.isLeader = false
					}

				} else {
					m.isLeader = evt.Status == leader.Leader
				}
				next := m.isLeader

				m.lock.Unlock()

				if current != next {
					notify <- next
				}

			}
		}

	}()

	return m.running, nil
}

// Stop stops the manager
func (m *manager) Stop() {
	if m.stop == nil {
		return
	}
	m.leader.Stop()
	close(m.stop)
}

func (m *manager) getCurrentState() (GlobalSpec, error) {
	// TODO(chungers) -- using the group plugin backend here isn't the general case.
	// When plugin activation is implemented, it's possible to have multiple group plugins
	// and the only way to reconstruct the GlobalSpec, which contains multiple groups of
	// possibly different group plugin implementations, is to do an 'all-shard' query across
	// all plugins of the type 'group' and then aggregate the results into the final GlobalSpec.
	// For now this just uses the gross simplification of asking the group plugin that the manager
	// proxies.

	global := GlobalSpec{
		Groups: map[group.ID]PluginSpec{},
	}

	specs, err := m.Plugin.InspectGroups()
	if err != nil {
		return global, err
	}

	for _, spec := range specs {
		buff, err := json.MarshalIndent(spec, "  ", "  ")
		if err != nil {
			return global, err
		}
		raw := json.RawMessage(buff)
		global.Groups[spec.ID] = PluginSpec{
			Plugin:     m.backendName,
			Properties: &raw,
		}
	}
	return global, nil
}

func (m *manager) onAssumeLeadership() error {
	log.Infoln("Assuming leadership")

	// load the config
	config := GlobalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err := m.snapshot.Load(&config)
	if err != nil {
		log.Warningln("Error loading config", err)
		return err
	}

	log.Infoln("Loaded snapshot. err=", err)
	if err != nil {
		return err
	}
	return m.doWatchGroups(config)
}

func (m *manager) onLostLeadership() error {
	log.Infoln("Lost leadership")
	config, err := m.getCurrentState()
	if err != nil {
		return err
	}
	return m.doUnwatchGroups(config)
}

func (m *manager) doCommit() error {

	// load the config
	config := GlobalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err := m.snapshot.Load(&config)
	if err != nil {
		return err
	}

	log.Infoln("Committing.  Loaded snapshot. err=", err)
	if err != nil {
		return err
	}
	return m.doUpdateGroups(config)
}

func (m *manager) doUpdateGroups(config GlobalSpec) error {
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Infoln("UPDATE group", spec.ID, "with spec:", spec)
			err := plugin.UpdateGroup(spec)

			// TODO(chungers) -- yes this is clunky comparing error text -- replace with typed error / code later.
			if err != nil && strings.Contains(err.Error(), "not being watched") {

				log.Infoln("UPDATE group", spec.ID, "changed to WATCH")
				err = plugin.WatchGroup(spec)

			}

			if err != nil {
				log.Warningln("Error updating/watch group:", spec.ID, "Err=", err)
			}
			return err
		})
}

func (m *manager) doWatchGroups(config GlobalSpec) error {
	log.Infoln("Start watching groups")
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Infoln("WATCH group", spec.ID, "with spec:", spec)
			err := plugin.WatchGroup(spec)

			// TODO(chungers) -- yes this is clunky with string comparison of error text.
			// Consider adding return code or error types for the Group SPI.
			if err != nil && strings.Contains(err.Error(), "Already watching") {

				log.Warningln("Already WATCHING", spec.ID, "no action")
				return nil
			}

			if err != nil {
				log.Warningln("Error watching group:", spec.ID, "Err=", err)
			}
			return nil
		})
}

func (m *manager) doUnwatchGroups(config GlobalSpec) error {
	log.Infoln("Unwatching groups")
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Infoln("UNWATCH group", spec.ID, "with spec:", spec)
			err := plugin.UnwatchGroup(spec.ID)
			if err != nil {
				log.Warningln("Error unwatching group:", spec.ID, "Err=", err)
			}
			return nil
		})
}

func (m *manager) execPlugins(config GlobalSpec, work func(group.Plugin, group.Spec) error) error {
	running, err := m.plugins.List()
	if err != nil {
		return err
	}

	for id, pluginSpec := range config.Groups {

		log.Infoln("Processing group", id, "with plugin", pluginSpec.Plugin)
		name := pluginSpec.Plugin

		ep, has := running[name]
		if !has {
			log.Warningln("Plugin", name, "isn't running")
			return err
		}

		gp, err := rpc.NewClient(ep.Protocol, ep.Address)
		if err != nil {
			log.Warningln("Cannot contact group", id, " at plugin", name, "endpoint=", ep.Address)
			return err
		}

		log.Debugln("exec on group", id, "plugin=", name)

		// spec is store in the properties
		if pluginSpec.Properties == nil {
			return fmt.Errorf("no spec for group %s plugin=%v", id, name)
		}

		spec := group.Spec{}
		err = json.Unmarshal([]byte(*pluginSpec.Properties), &spec)
		if err != nil {
			return err
		}

		err = work(gp, spec)
		if err != nil {
			log.Warningln("Error from exec on plugin", err)
			return err
		}

	}

	return nil
}
