package manager

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	group_plugin "github.com/docker/infrakit/pkg/plugin/group"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// manager is the controller of all the plugins.  It is able to process multiple inputs
// such as leadership changes and configuration changes and perform the necessary actions
// to activate / deactivate plugins
type manager struct {

	// Options include configurations
	Options

	// Some methods are overridden to provide persistence services
	group.Plugin

	// Some methods are overridden to provide persistence services
	metadata.Updatable

	isLeader bool
	lock     sync.Mutex
	stop     chan struct{}
	running  chan struct{}

	// Status is the status metadata (readonly)
	Status            metadata.Plugin
	doneStatusUpdates chan struct{}

	// queued operations
	backendOps chan<- backendOp
}

const (
	// defaultPluginPollInterval is the interval to retry connection
	defaultPluginPollInterval = 2 * time.Second
)

type backendOp struct {
	name      string
	operation func() error
}

// NewManager returns the manager which depends on other services to coordinate and manage
// the plugins in order to ensure the infrastructure state matches the user's spec.
func NewManager(options Options) Backend {

	if options.MetadataStore == nil {
		log.Warn("no metadata store. nothing will be persisted")
	}

	impl := &manager{
		Options: options,
		// "base class" is the stateless backend group plugin
		Plugin: group_plugin.LazyConnect(
			func() (group.Plugin, error) {

				endpoint, err := options.Plugins().Find(options.Group)
				if err != nil {
					return nil, err
				}
				return group_rpc.NewClient(endpoint.Address)
			}, defaultPluginPollInterval),
		Updatable: initUpdatable(options),
	}

	impl.Status = initStatusMetadata(impl)
	return impl
}

func initUpdatable(options Options) metadata.Updatable {

	data := map[string]interface{}{}

	writer := func(proposed *types.Any) error {
		// write
		log.Debug("updating", "proposed", proposed)
		if options.MetadataStore != nil {
			var v interface{}
			err := proposed.Decode(&v)
			if err != nil {
				return err
			}
			log.Debug("saving", "proposed", proposed)
			return options.MetadataStore.Save(v)
		}
		return proposed.Decode(&data)
	}

	// There's a chance the metadata is readonly.  In this case, we want to provide
	// persistence services for this plugin and make it an Updatable
	return metadata_plugin.UpdatableLazyConnect(
		func() (metadata.Updatable, error) {

			log.Debug("looking up metadata backend", "name", options.Metadata)

			var p metadata.Plugin
			if options.Metadata.IsEmpty() {
				// in-memory only
				p = metadata_plugin.NewUpdatablePlugin(metadata_plugin.NewPluginFromData(data), writer)

				log.Info("backend metadata is in memory only")

			} else {

				endpoint, err := options.Plugins().Find(options.Metadata)
				if err != nil {
					return nil, err
				}
				found, err := metadata_rpc.NewClient(options.Metadata, endpoint.Address)
				if err != nil {
					return nil, err
				}
				p = found

				_, is := p.(metadata.Updatable)
				log.Info("backend metadata", "name", options.Metadata, "plugin", p, "updatable", is)

			}
			return metadata_plugin.NewUpdatablePlugin(p, writer), nil
		}, defaultPluginPollInterval)

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
	if m.Options.LeaderStore == nil {
		return nil, fmt.Errorf("cannot locate leader")
	}

	return m.Options.LeaderStore.GetLocation()
}

// Start starts the manager.  It does not block. Instead read from the returned channel to block.
func (m *manager) Start() (<-chan struct{}, error) {

	// initRunning guarantees that the m.running will be initialized the first time it's
	// called.  If another call of Start is made after the first, don't do anything just return the references.
	if !m.initRunning() {
		return m.running, nil
	}

	log.Info("Manager starting")

	// Refreshes metadata
	metadataPoll := m.Options.MetadataRefreshInterval.Duration()
	if metadataPoll > 0 {
		metadataRefresh := time.Tick(metadataPoll)
		go func() {
			for {
				// We load the metadata from the backend so
				// that we have the latest. This is important
				// in case that writes are performed on the same
				// leader by another process.
				// Note that we use optimistic concurrency of
				// computing hashes from each change and each batch
				// of changes require reading the entire data struct
				// so this constant updating should not cause problem
				// if writes are performed from another process on the
				// same leader node.
				err := m.loadMetadata()
				if err != nil {
					log.Debug("error loading metadata", "err", err)
				}

				<-metadataRefresh
			}
		}()
	}

	leaderChan, err := m.Options.Leader.Start()
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
	close(m.doneStatusUpdates)
	close(m.stop)
	m.Options.Leader.Stop()
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
		global.updateGroupSpec(spec, m.Options.Group)
	}
	return global, nil
}

func (m *manager) loadAndCommitSpecs() error {
	if m.Options.SpecStore == nil {
		return nil
	}

	log.Info("Loading specs and committing")
	// load the config
	config := &globalSpec{}
	// load the latest version -- assumption here is that it's been persisted already.
	log.Info("Loading snapshot")
	err := config.load(m.Options.SpecStore)
	log.Info("Loaded snapshot", "err", err)
	if err != nil {
		log.Warn("Error loading config", "err", err)
		return err
	}
	return m.doCommitGroups(*config)
}

func (m *manager) loadMetadata() (err error) {
	if m.Options.MetadataStore == nil {
		return nil
	}

	log.Debug("loading metadata and committing", "V", debugV2)

	var saved interface{}
	err = m.Options.MetadataStore.Load(&saved)
	if err != nil {
		return
	}

	any, e := types.AnyValue(saved)
	if e != nil {
		err = e
		return
	}

	if any == nil {
		log.Debug("no metadata stored", "V", debugV2)
		return
	}

	log.Debug("loaded metadata", "data", any.String(), "V", debugV2)
	_, proposed, cas, e := m.Updatable.Changes([]metadata.Change{
		{Path: types.Dot, Value: any},
	})
	if e != nil {
		log.Error("trying to update metadata", "err", e)
		err = e
		return
	}

	log.Debug("updating backend with stored metadata", "cas", cas, "proposed", proposed, "V", debugV2)
	return m.Updatable.Commit(proposed, cas)
}

func (m *manager) onAssumeLeadership() (err error) {
	log.Info("Assuming leadership")

	defer log.Info("Running as leader")

	err = m.loadMetadata()
	if err != nil {
		log.Error("error loading metadata", "err", err)
	}

	err = m.loadAndCommitSpecs()
	log.Debug("Loading and committing specs", "err", err)

	if err != nil && m.Options.LeaderCommitSpecsRetries > 0 {

		log.Info("Retry loading and committing specs",
			"retries", m.Options.LeaderCommitSpecsRetries,
			"interval", m.Options.LeaderCommitSpecsRetryInterval)

		delay := 1 * time.Second
		if m.Options.LeaderCommitSpecsRetryInterval > 0 {
			delay = m.Options.LeaderCommitSpecsRetryInterval.Duration()
		}

		for i := 1; i < m.Options.LeaderCommitSpecsRetries; i++ {

			<-time.After(delay)

			err = m.loadAndCommitSpecs()
			if err == nil {
				log.Info("Loaded and committed specs")
				return nil
			}

			if err != nil {
				log.Error("error loading specs", "err", err, "attempt", i)
			}

		}

	}

	return err
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
	err := config.load(m.Options.SpecStore)
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
	running, err := m.Options.Plugins().List()
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

		gp, err := group_rpc.NewClient(ep.Address)
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
