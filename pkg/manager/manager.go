package manager

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log = logutil.New("module", "manager")

	debugV  = logutil.V(100)
	debugV2 = logutil.V(500)

	// InterfaceSpec is the current name and version of the Instance API.
	InterfaceSpec = spi.InterfaceSpec{
		Name:    "Manager",
		Version: "0.1.0",
	}
)

// Leadership is the interface for getting information about the current leader node
type Leadership interface {
	// IsLeader returns true only if for certain this is a leader. False if not or unknown.
	IsLeader() (bool, error)
}

// Manager is the interface for interacting locally or remotely with the manager
type Manager interface {
	Leadership

	// LeaderLocation returns the location of the leader
	LeaderLocation() (*url.URL, error)

	// Enforce enforces infrastructure state to match that of the specs
	Enforce(specs []types.Spec) error

	// Inspect returns the current state of the infrastructure
	Inspect() ([]types.Object, error)

	// Terminate destroys all resources associated with the specs
	Terminate(specs []types.Spec) error
}

// Backend is the admin / server interface
type Backend interface {
	group.Plugin

	Controllers() (map[string]controller.Controller, error)
	Groups() (map[group.ID]group.Plugin, error)

	Manager

	Start() (<-chan struct{}, error)
	Stop()
}

// manager is the controller of all the plugins.  It is able to process multiple inputs
// such as leadership changes and configuration changes and perform the necessary actions
// to activate / deactivate plugins
type manager struct {
	group.Plugin // Note that some methods are overridden

	plugins     discovery.Plugins
	leader      leader.Detector
	leaderStore leader.Store
	snapshot    store.Snapshot
	isLeader    bool
	lock        sync.Mutex
	stop        chan struct{}
	running     chan struct{}

	backendName string
	backendOps  chan<- backendOp
}

type backendOp struct {
	name      string
	operation func() error
}

// NewManager returns the manager which depends on other services to coordinate and manage
// the plugins in order to ensure the infrastructure state matches the user's spec.
func NewManager(plugins discovery.Plugins,
	leader leader.Detector,
	leaderStore leader.Store,
	snapshot store.Snapshot,
	backendName string) Backend {

	return &manager{
		// "base class" is the stateless backend group plugin
		Plugin: &lateBindGroup{
			finder: func() (group.Plugin, error) {
				endpoint, err := plugins.Find(plugin.Name(backendName))
				if err != nil {
					return nil, err
				}
				return rpc.NewClient(endpoint.Address)
			},
		},
		plugins:     plugins,
		leader:      leader,
		leaderStore: leaderStore,
		snapshot:    snapshot,
		backendName: backendName,
	}
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

// LeaderLocation returns the location of the leader
func (m *manager) LeaderLocation() (*url.URL, error) {
	if m.leaderStore == nil {
		return nil, fmt.Errorf("cannot locate leader")
	}

	return m.leaderStore.GetLocation()
}

// Enforce enforces infrastructure state to match that of the specs
func (m *manager) Enforce(specs []types.Spec) error {

	buff, err := types.AnyValueMust(specs).MarshalYAML()
	if err != nil {
		return err
	}

	fmt.Println(string(buff))

	return nil
}

// Inspect returns the current state of the infrastructure
func (m *manager) Inspect() ([]types.Object, error) {
	return nil, nil
}

// Terminate destroys all resources associated with the specs
func (m *manager) Terminate(specs []types.Spec) error {
	return fmt.Errorf("not implemented")
}

// Start starts the manager.  It does not block. Instead read from the returned channel to block.
func (m *manager) Start() (<-chan struct{}, error) {

	// initRunning guarantees that the m.running will be initialized the first time it's
	// called.  If another call of Start is made after the first, don't do anything just return the references.
	if !m.initRunning() {
		return m.running, nil
	}

	log.Info("Manager starting")

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
				log.Debug("Backend operation", "op", op, "V", debugV)
				if m.isLeader {
					op.operation()
				}

			case <-stopWorkQueue:

				log.Info("Stopping work queue.")
				close(m.running)
				log.Info("Manager stopped.")
				return

			case leader, open := <-notify:

				if !open {
					return
				}

				// This channel has data only when there's been a leadership change.

				log.Debug("leader event", "leader", leader)
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

				log.Info("Stopping..")
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
					log.Warn("Leadership status is uncertain", "err", evt.Error)

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

	global := globalSpec{}

	specs, err := m.Plugin.InspectGroups()
	if err != nil {
		return global, err
	}

	for _, spec := range specs {
		global.updateGroupSpec(spec, plugin.Name(m.backendName))
	}
	return global, nil
}

func (m *manager) onAssumeLeadership() error {
	log.Info("Assuming leadership")

	// load the config
	config := &globalSpec{}
	// load the latest version -- assumption here is that it's been persisted already.
	log.Info("Loading snapshot")
	err := config.load(m.snapshot)
	log.Info("Loaded snapshot", "err", err)
	if err != nil {
		log.Warn("Error loading config", "err", err)
		return err
	}
	return m.doCommitGroups(*config)
}

func (m *manager) onLostLeadership() error {
	log.Info("Lost leadership")
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
	err := config.load(m.snapshot)
	if err != nil {
		return err
	}

	log.Info("Committing. Loaded snapshot.", "err", err)
	if err != nil {
		return err
	}
	return m.doCommitGroups(config)
}

func (m *manager) doCommitGroups(config globalSpec) error {
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Info("Committing group", "groupID", spec.ID, "spec", spec)

			_, err := plugin.CommitGroup(spec, false)
			if err != nil {
				log.Warn("Error committing group.", "groupID", spec.ID, "err", err)
			}
			return err
		})
}

func (m *manager) doFreeGroups(config globalSpec) error {
	log.Info("Freeing groups")
	return m.execPlugins(config,
		func(plugin group.Plugin, spec group.Spec) error {

			log.Info("Freeing group", "groupID", spec.ID)
			err := plugin.FreeGroup(spec.ID)
			if err != nil {
				log.Warn("Error freeing group", "groupID", spec.ID, "err", err)
			}
			return nil
		})
}

func (m *manager) execPlugins(config globalSpec, work func(group.Plugin, group.Spec) error) error {
	running, err := m.plugins.List()
	if err != nil {
		return err
	}

	return config.visit(func(k key, r record) error {

		// TODO(chungers) ==> temporary
		if k.Kind != "group" {
			return nil
		}

		id := group.ID(k.Name)

		lookup, _ := r.Handler.GetLookupAndType()
		log.Debug("Processing group", "groupID", id, "plugin", r.Handler, "V", logutil.V(100))

		ep, has := running[lookup]
		if !has {
			log.Warn("Not running", "plugin", lookup, "name", r.Handler)
			return err
		}

		gp, err := rpc.NewClient(ep.Address)
		if err != nil {
			log.Warn("Cannot contact group", "groupID", id, "plugin", r.Handler, "endpoint", ep.Address)
			return err
		}

		log.Debug("exec on group", "groupID", id, "plugin", r.Handler, "V", logutil.V(100))

		// spec is store in the properties
		if r.Spec.Properties == nil {
			return fmt.Errorf("no spec for group %s plugin=%v", id, r.Handler)
		}

		spec := group.Spec{
			ID:         id,
			Properties: r.Spec.Properties,
		}

		err = work(gp, spec)
		if err != nil {
			log.Warn("Error from exec on plugin", "err", err)
			return err
		}

		return nil
	})
}
