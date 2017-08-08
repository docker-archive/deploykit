package manager

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin and also key used to locate the plugin in discovery
	CanonicalName = "manager"

	// LookupName is the name used to look up the object via discovery
	LookupName = "group"

	// EnvOptionsBackend is the environment variable to use to set the default value of Options.Backend
	EnvOptionsBackend = "INFRAKIT_MANAGER_OPTIONS_BACKEND"
)

var (
	log                   = logutil.New("module", "run/manager")
	defaultOptionsBackend = run.GetEnv(EnvOptionsBackend, "file")
)

func init() {
	inproc.Register(CanonicalName, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Backend is the backend used for leadership, persistence, etc.
	// Possible values are file, etcd, and swarm
	Backend string

	// Name of the backend
	BackendName plugin.Name

	// Settings is the configuration of the backend
	Settings *types.Any
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = defaultOptions()

func defaultOptions() (options Options) {
	b := os.Getenv(EnvOptionsBackend)
	switch b {
	case "swarm":
		options = DefaultBackendSwarmOptions
	case "etcd":
		options = DefaultBackendEtcdOptions
	case "file":
		options = DefaultBackendFileOptions
	default:
		options = DefaultBackendFileOptions
	}

	options.BackendName = plugin.Name("group-stateless")
	return options
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins,
	config *types.Any) (name plugin.Name, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	if plugins == nil {
		panic("no plugins()")
	}

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	log.Info("Decoded input", "config", options)
	log.Info("Starting up", "backend", options.Backend)

	var leader leader.Detector
	var snapshot store.Snapshot
	var cleanUpFunc func()

	switch strings.ToLower(options.Backend) {
	case "etcd":
		backendOptions := BackendEtcdOptions{}
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up etcd backend", "options", backendOptions)
		leader, snapshot, cleanUpFunc, err = etcdBackends(backendOptions)
		if err != nil {
			return
		}
		log.Info("etcd backend", "leader", leader, "store", snapshot, "cleanup", cleanUpFunc)
	case "file":
		backendOptions := BackendFileOptions{}
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up file backend", "options", backendOptions)
		leader, snapshot, cleanUpFunc, err = fileBackends(backendOptions)
		if err != nil {
			return
		}
		log.Info("file backend", "leader", leader, "store", snapshot, "cleanup", cleanUpFunc)
	case "swarm":
		backendOptions := BackendSwarmOptions{}
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up swarm backend", "options", backendOptions)
		leader, snapshot, cleanUpFunc, err = swarmBackends(backendOptions)
		if err != nil {
			return
		}
		log.Info("swarm backend", "leader", leader, "store", snapshot, "cleanup", cleanUpFunc)
	default:
		err = fmt.Errorf("unknown backend:%v", options.Backend)
		return
	}

	var mgr manager.Backend
	lookup, _ := options.BackendName.GetLookupAndType()
	mgr, err = manager.NewManager(plugins(), leader, snapshot, lookup)
	if err != nil {
		return
	}

	log.Info("start manager")

	_, err = mgr.Start()
	if err != nil {
		return
	}

	log.Info("manager running")

	updatable := &metadataModel{
		snapshot: snapshot,
		manager:  mgr,
	}
	updatableModel, _ := updatable.pluginModel()
	stopRelay := make(chan struct{})
	copy1 := make(chan func(map[string]interface{}))
	copy2 := make(chan func(map[string]interface{}))
	go func() {
		// relay data
		for {
			select {
			case data := <-updatableModel:
				copy1 <- data
				copy2 <- data
			case <-stopRelay:
				return // finished
			}
		}
	}()

	name = plugin.Name(LookupName)

	metadataUpdatable := metadata_plugin.NewUpdatablePlugin(
		metadata_plugin.NewPluginFromChannel(updatableModel),
		updatable.load, updatable.commit)

	impls = map[run.PluginCode]interface{}{
		run.Manager:           mgr,
		run.Group:             mgr,
		run.MetadataUpdatable: metadataUpdatable,
		run.Metadata:          metadataUpdatable,
	}

	onStop = func() {
		if cleanUpFunc != nil {
			cleanUpFunc()
		}
		close(stopRelay)
	}

	log.Info("exported objects")
	return
}

type cleanup func()
