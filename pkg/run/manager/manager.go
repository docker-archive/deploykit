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
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin and also key used to locate the plugin in discovery
	CanonicalName = "manager"

	// EnvOptionsBackend is the environment variable to use to set the default value of Options.Backend
	EnvOptionsBackend = "INFRAKIT_MANAGER_OPTIONS_BACKEND"
)

var (
	log                   = logutil.New("module", "run/manager")
	defaultOptionsBackend = os.Getenv(EnvOptionsBackend)
)

func init() {
	if defaultOptionsBackend == "" {
		defaultOptionsBackend = "file"
	}

	inproc.Register("instance-manager", Run, DefaultOptions)
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
	config *types.Any) (name plugin.Name, impls []interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	var leader leader.Detector
	var snapshot store.Snapshot
	var cleanUp func()

	switch strings.ToLower(options.Backend) {
	case "etcd":
		backendOptions := BackendEtcdOptions{}
		err = config.Decode(&backendOptions)
		if err != nil {
			return
		}
		leader, snapshot, cleanUp, err = etcdBackends(backendOptions)
	case "file":
		backendOptions := BackendFileOptions{}
		err = config.Decode(&backendOptions)
		if err != nil {
			return
		}
		leader, snapshot, cleanUp, err = fileBackends(backendOptions)
	case "swarm":
		backendOptions := BackendSwarmOptions{}
		err = config.Decode(&backendOptions)
		if err != nil {
			return
		}
		leader, snapshot, cleanUp, err = swarmBackends(backendOptions)
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

	_, err = mgr.Start()
	if err != nil {
		return
	}

	updatable := &metadataModel{
		snapshot: snapshot,
		manager:  mgr,
	}
	updatableModel, stopUpdatable := updatable.pluginModel()
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
				close(stopUpdatable)
			}
		}
	}()
	name = plugin.Name(CanonicalName)
	impls = []interface{}{
		mgr,
		mgr.(group.Plugin),
		metadata_plugin.NewUpdatablePlugin(
			metadata_plugin.NewPluginFromChannel(copy1),
			updatable.load, updatable.commit),
		metadata_plugin.NewPluginFromChannel(copy2),
	}
	onStop = func() {
		cleanUp()
		close(stopRelay)
	}
	return
}

type cleanup func()
