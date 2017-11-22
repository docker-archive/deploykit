package manager

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// manager is the controller of all the plugins.  It is able to process multiple inputs
// such as leadership changes and configuration changes and perform the necessary actions
// to activate / deactivate plugins
type manager struct {
	scope scope.Scope

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
func NewManager(scope scope.Scope, options Options) Backend {

	if options.MetadataStore == nil {
		log.Warn("no metadata store. nothing will be persisted")
	}

	gp, _ := scope.Group(options.Group.String())

	impl := &manager{
		scope:     scope,
		Options:   options,
		Plugin:    gp, // the stateless backend group plugin
		Updatable: initUpdatable(scope, options),
	}

	impl.Status = initStatusMetadata(impl)
	return impl
}

func initUpdatable(scope scope.Scope, options Options) metadata.Updatable {

	data := map[string]interface{}{}

	writer := func(proposed *types.Any) error {
		// write
		log.Debug("updating", "proposed", proposed, "V", debugV3)
		if options.MetadataStore != nil {
			var v interface{}
			err := proposed.Decode(&v)
			if err != nil {
				return err
			}
			log.Debug("saving", "proposed", proposed, "V", debugV3)
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

				metadataCall, err := scope.Metadata(options.Metadata.String())
				if err != nil {
					return nil, err
				}
				p = metadataCall.Plugin

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
	return m.doCommitAll(*config)
}

func (m *manager) loadMetadata() (err error) {
	if m.Options.MetadataStore == nil {
		return nil
	}

	log.Debug("loading metadata and committing", "V", debugV3)

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
		log.Debug("no metadata stored", "V", debugV3)
		return
	}

	log.Debug("loaded metadata", "data", any.String(), "V", debugV3)
	_, proposed, cas, e := m.Updatable.Changes([]metadata.Change{
		{Path: types.Dot, Value: any},
	})
	if e != nil {
		log.Error("trying to update metadata", "err", e)
		err = e
		return
	}

	log.Debug("updating backend with stored metadata", "cas", cas, "proposed", proposed, "V", debugV3)
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
	return m.doFreeAll(config)
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
	return m.doCommitAll(config)
}

func (m *manager) doCommitAll(config globalSpec) error {
	return m.execPlugins(config,
		func(control controller.Controller, spec types.Spec) error {

			log.Info("Committing spec", "spec", spec)

			_, err := control.Commit(controller.Enforce, spec)
			return err
		},
		func(plugin group.Plugin, spec group.Spec) error {

			log.Info("Committing group", "groupID", spec.ID, "spec", spec)

			_, err := plugin.CommitGroup(spec, false)
			return err
		})
}

func (m *manager) doFreeAll(config globalSpec) error {
	log.Info("Freeing groups")
	return m.execPlugins(config,
		func(controller controller.Controller, spec types.Spec) error {

			log.Info("Freeing spec", "spec", spec)

			_, err := controller.Free(&spec.Metadata)
			return err
		},
		func(plugin group.Plugin, spec group.Spec) error {

			log.Info("Freeing group", "groupID", spec.ID)
			return plugin.FreeGroup(spec.ID)
		})
}

func (m *manager) execPlugins(config globalSpec,
	controllerWork func(controller.Controller, types.Spec) error,
	groupWork func(group.Plugin, group.Spec) error) (err error) {

	return config.visit(func(k key, r record) error {

		// TODO(chungers) ==> temporary
		switch k.Kind {
		case "ingress", "enrollment":

			cp, err := m.scope.Controller(r.Handler.String())
			if err != nil {
				log.Error("Error getting controller", "plugin", r.Handler, "err", err)
				break
			}

			err = controllerWork(cp, r.Spec)

		case "group": // not ideal to use string here.
			id := group.ID(k.Name)
			gp, err := m.scope.Group(r.Handler.String())
			if err != nil {
				log.Error("Cannot contact group", "groupID", id, "plugin", r.Handler)
				break
			}

			log.Debug("exec on group", "groupID", id, "plugin", r.Handler, "V", logutil.V(100))

			// spec is store in the properties
			if r.Spec.Properties == nil {
				err = fmt.Errorf("no spec for group %s plugin=%v", id, r.Handler)
				break
			}

			spec := group.Spec{
				ID:         id,
				Properties: r.Spec.Properties,
			}

			err = groupWork(gp, spec)

		default:
			log.Warn("not execing on for record", "record", r, "key", k)
			return nil
		}

		if err != nil {
			log.Error("Error from exec on plugin", "err", err)
		}
		return err

	})
}
