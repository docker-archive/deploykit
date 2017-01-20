package manager

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

// InterfaceSpec is the current name and version of the Instance API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Manager",
	Version: "0.1.0",
}

// Manager is the interface for interacting locally or remotely with the manager
type Manager interface {
	// IsLeader returns true only if for certain this is a leader. False if not or unknown.
	IsLeader() (bool, error)
}

// Backend is the admin / server interface
type Backend interface {
	group.Plugin

	Manager

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
}

// NewManager returns the manager which depends on other services to coordinate and manage
// the plugins in order to ensure the infrastructure state matches the user's spec.
func NewManager(
	plugins discovery.Plugins,
	leader leader.Detector,
	snapshot store.Snapshot,
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

	if m.running == nil {
		m.running = make(chan struct{})
		return true
	}
	return false
}

// IsLeader returns leader status.  False if not or unknown.
func (m *manager) IsLeader() (bool, error) {
	return m.isLeader, nil
}

// Start starts the manager.  It does not block. Instead read from the returned channel to block.
func (m *manager) Start() (<-chan struct{}, error) {

	// initRunning guarantees that the m.running will be initialized the first time it's
	// called.  If another call of Start is made after the first, don't do anything just return the references.
	if !m.initRunning() {
		return m.running, nil
	}

	log.Infoln("Manager starting")

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
					op.operation()
				}

			case <-stopWorkQueue:

				log.Infoln("Stopping work queue.")
				close(m.running)
				log.Infoln("Manager stopped.")
				return

			case leader, open := <-notify:

				if !open {
					return
				}

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
				m.stop = nil
				close(stopWorkQueue)
				close(notify)
				return

			case evt, open := <-leaderChan:

				if !open {
					return
				}

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
	close(m.stop)
	m.leader.Stop()
}

func (m *manager) getCurrentState() (globalSpec, error) {
	// TODO(chungers) -- using the group plugin backend here isn't the general case.
	// When plugin activation is implemented, it's possible to have multiple group plugins
	// and the only way to reconstruct the globalSpec, which contains multiple groups of
	// possibly different group plugin implementations, is to do an 'all-shard' query across
	// all plugins of the type 'group' and then aggregate the results into the final globalSpec.
	// For now this just uses the gross simplification of asking the group plugin that the manager
	// proxies.

	global := globalSpec{
		Groups: map[group.ID]plugin.Spec{},
	}

	specs, err := m.Plugin.InspectGroups()
	if err != nil {
		return global, err
	}

	for _, spec := range specs {

		any, err := types.AnyValue(spec)
		if err != nil {
			return global, err
		}
		global.Groups[spec.ID] = plugin.Spec{
			Plugin:     plugin.Name(m.backendName),
			Properties: any,
		}
	}
	return global, nil
}

func (m *manager) onAssumeLeadership() error {
	log.Infoln("Assuming leadership")

	// load the config
	config := globalSpec{}

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
	return m.doCommitGroups(config)
}

func (m *manager) onLostLeadership() error {
	log.Infoln("Lost leadership")
	config, err := m.getCurrentState()
	if err != nil {
		return err
	}
	return m.doFreeGroups(config)
}

func (m *manager) doCommit() error {

	// load the config
	config := globalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err := m.snapshot.Load(&config)
	if err != nil {
		return err
	}

	log.Infoln("Committing. Loaded snapshot. err=", err)
	if err != nil {
		return err
	}
	return m.doCommitGroups(config)
}

func (m *manager) doCommitGroups(config globalSpec) error {
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Infoln("Committing group", spec.ID, "with spec:", spec)

			_, err := plugin.CommitGroup(spec, false)
			if err != nil {
				log.Warningln("Error committing group:", spec.ID, "Err=", err)
			}
			return err
		})
}

func (m *manager) doFreeGroups(config globalSpec) error {
	log.Infoln("Freeing groups")
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Infoln("Freeing group", spec.ID)
			err := plugin.FreeGroup(spec.ID)
			if err != nil {
				log.Warningln("Error freeing group:", spec.ID, "Err=", err)
			}
			return nil
		})
}

func (m *manager) execPlugins(config globalSpec, work func(group.Plugin, group.Spec) error) error {
	running, err := m.plugins.List()
	if err != nil {
		return err
	}

	for id, pluginSpec := range config.Groups {

		log.Infoln("Processing group", id, "with plugin", pluginSpec.Plugin)
		name := pluginSpec.Plugin.String()

		ep, has := running[name]
		if !has {
			log.Warningln("Plugin", name, "isn't running")
			return err
		}

		gp, err := rpc.NewClient(ep.Address)
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
