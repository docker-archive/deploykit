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
	"github.com/docker/infrakit/pkg/rpc/mux"
	rpc "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin and also key used to locate the plugin in discovery
	Kind = "manager"

	// LookupName is the name used to look up the object via discovery
	LookupName = "group"

	// EnvOptionsBackend is the environment variable to use to set the default value of Options.Backend
	EnvOptionsBackend = "INFRAKIT_MANAGER_BACKEND"

	// EnvMuxListen is the listen string (:24864)
	EnvMuxListen = "INFRAKIT_MUX_LISTEN"

	// EnvAdvertise is the location of this node (127.0.0.1:24864)
	EnvAdvertise = "INFRAKIT_ADVERTISE"
)

var (
	log                   = logutil.New("module", "run/manager")
	defaultOptionsBackend = run.GetEnv(EnvOptionsBackend, "file")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
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

	// Mux is the tcp frontend for remote connectivity
	Mux *MuxConfig

	plugins     func() discovery.Plugins
	leader      leader.Detector
	leaderStore leader.Store
	store       store.Snapshot
	cleanUpFunc func()
}

// MuxConfig is the struct for the mux frontend
type MuxConfig struct {
	// Listen string e.g. :24864
	Listen string

	// Advertise is the public listen string e.g. public_ip:24864
	Advertise string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = defaultOptions()

func defaultOptions() (options Options) {

	options = Options{
		BackendName: plugin.Name("group-stateless"),
		Mux: &MuxConfig{
			Listen:    run.GetEnv(EnvMuxListen, ":24864"),
			Advertise: run.GetEnv(EnvAdvertise, "localhost:24864"),
		},
	}

	options.Backend = os.Getenv(EnvOptionsBackend)
	switch options.Backend {
	case "swarm":
		options.Backend = "swarm"
		options.Settings = DefaultBackendSwarmOptions
	case "etcd":
		options.Backend = "etcd"
		options.Settings = DefaultBackendEtcdOptions
	case "file":
		options.Backend = "file"
		options.Settings = DefaultBackendFileOptions
	default:
		options.Backend = "file"
		options.Settings = DefaultBackendFileOptions
	}

	return
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	if plugins == nil {
		panic("no plugins()")
	}

	options := Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	log.Info("Decoded input", "config", options)
	log.Info("Starting up", "backend", options.Backend)

	options.plugins = plugins

	switch strings.ToLower(options.Backend) {
	case "etcd":
		backendOptions := BackendEtcdOptions{}
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up etcd backend", "options", backendOptions)
		err = configEtcdBackends(backendOptions, &options)
		if err != nil {
			return
		}
		log.Info("etcd backend", "leader", options.leader, "store", options.store, "cleanup", options.cleanUpFunc)
	case "file":
		backendOptions := BackendFileOptions{}
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up file backend", "options", backendOptions)
		err = configFileBackends(backendOptions, &options)
		if err != nil {
			return
		}
		log.Info("file backend", "leader", options.leader, "store", options.store, "cleanup", options.cleanUpFunc)
	case "swarm":
		backendOptions := BackendSwarmOptions{}
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up swarm backend", "options", backendOptions)
		err = configSwarmBackends(backendOptions, &options)
		if err != nil {
			return
		}
		log.Info("swarm backend", "leader", options.leader, "store", options.store, "cleanup", options.cleanUpFunc)
	default:
		err = fmt.Errorf("unknown backend:%v", options.Backend)
		return
	}

	var mgr manager.Backend
	lookup, _ := options.BackendName.GetLookupAndType()
	mgr, err = manager.NewManager(plugins(), options.leader, options.leaderStore, options.store, lookup)
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
		snapshot: options.store,
		manager:  mgr,
	}
	updatableModel, _ := updatable.pluginModel()

	transport.Name = name

	metadataUpdatable := metadata_plugin.NewUpdatablePlugin(
		metadata_plugin.NewPluginFromChannel(updatableModel),
		updatable.load, updatable.commit)

	impls = map[run.PluginCode]interface{}{
		run.Manager:           mgr,
		run.Controller:        mgr.GroupControllers,
		run.Group:             mgr.Groups,
		run.MetadataUpdatable: metadataUpdatable,
		run.Metadata:          metadataUpdatable,
	}

	var muxServer rpc.Stoppable

	if options.Mux != nil {

		log.Info("Starting mux server", "listen", options.Mux.Listen, "advertise", options.Mux.Advertise)
		muxServer, err = mux.NewServer(options.Mux.Listen, options.Mux.Advertise, options.plugins,
			mux.Options{
				Leadership: options.leader.Receive(),
				Registry:   options.leaderStore,
			})
		if err != nil {
			panic(err)
		}
	}

	onStop = func() {
		if options.cleanUpFunc != nil {
			options.cleanUpFunc()
		}
		if muxServer != nil {
			muxServer.Stop()
		}
	}

	log.Info("exported objects")
	return
}

type cleanup func()
