package manager

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/rpc/mux"
	rpc "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
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
	defaultOptionsBackend = local.Getenv(EnvOptionsBackend, "file")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	manager.Options

	// Backend is the backend used for leadership, persistence, etc.
	// Possible values are file, etcd, and swarm
	Backend string

	// Settings is the configuration of the backend
	Settings *types.Any

	// Mux is the tcp frontend for remote connectivity
	Mux *MuxConfig

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
		Options: manager.Options{
			Group:    plugin.Name("group-stateless"),
			Metadata: plugin.Name("vars-stateless"),
		},
		Mux: &MuxConfig{
			Listen:    local.Getenv(EnvMuxListen, ":24864"),
			Advertise: local.Getenv(EnvAdvertise, "localhost:24864"),
		},
	}

	options.Backend = os.Getenv(EnvOptionsBackend)
	switch options.Backend {
	case "swarm":
		options.Backend = "swarm"
		options.Settings = types.AnyValueMust(DefaultBackendSwarmOptions)
	case "etcd":
		options.Backend = "etcd"
		options.Settings = types.AnyValueMust(DefaultBackendEtcdOptions)
	case "file":
		options.Backend = "file"
		options.Settings = types.AnyValueMust(DefaultBackendFileOptions)
	default:
		options.Backend = "file"
		options.Settings = types.AnyValueMust(DefaultBackendFileOptions)
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

	options := defaultOptions()
	err = config.Decode(&options)
	if err != nil {
		return
	}

	log.Info("Decoded input", "config", options)
	log.Info("Starting up", "backend", options.Backend)

	options.Name = name
	options.Plugins = plugins

	switch strings.ToLower(options.Backend) {
	case "etcd":
		backendOptions := DefaultBackendEtcdOptions
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up etcd backend", "options", backendOptions)
		err = configEtcdBackends(backendOptions, &options)
		if err != nil {
			return
		}
		log.Info("etcd backend", "leader", options.Leader, "store", options.SpecStore, "cleanup", options.cleanUpFunc)
	case "file":
		backendOptions := DefaultBackendFileOptions
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up file backend", "options", backendOptions)
		err = configFileBackends(backendOptions, &options)
		if err != nil {
			return
		}
		log.Info("file backend", "leader", options.Leader, "store", options.SpecStore, "cleanup", options.cleanUpFunc)
	case "swarm":
		backendOptions := DefaultBackendSwarmOptions
		err = options.Settings.Decode(&backendOptions)
		if err != nil {
			return
		}
		log.Info("starting up swarm backend", "options", backendOptions)
		err = configSwarmBackends(backendOptions, &options)
		if err != nil {
			return
		}
		log.Info("swarm backend", "leader", options.Leader, "store", options.SpecStore, "cleanup", options.cleanUpFunc)
	default:
		err = fmt.Errorf("unknown backend:%v", options.Backend)
		return
	}

	mgr := manager.NewManager(options.Options)
	log.Info("Start manager", "m", mgr)

	_, err = mgr.Start()
	if err != nil {
		return
	}

	log.Info("Manager running")

	updatable := &metadataModel{
		snapshot: options.SpecStore,
		manager:  mgr,
	}
	updatableModel, _ := updatable.pluginModel()

	transport.Name = name

	metadataUpdatable := metadata_plugin.NewUpdatablePlugin(
		metadata_plugin.NewPluginFromChannel(updatableModel), updatable.commit)

	impls = map[run.PluginCode]interface{}{
		run.Manager:           mgr,
		run.Controller:        mgr.Controllers,
		run.Group:             mgr.Groups,
		run.MetadataUpdatable: metadataUpdatable,
		run.Metadata:          metadataUpdatable,
	}

	var muxServer rpc.Stoppable

	if options.Mux != nil {

		log.Info("Starting mux server", "listen", options.Mux.Listen, "advertise", options.Mux.Advertise)
		muxServer, err = mux.NewServer(options.Mux.Listen, options.Mux.Advertise, options.Plugins,
			mux.Options{
				Leadership: options.Leader.Receive(),
				Registry:   options.LeaderStore,
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
