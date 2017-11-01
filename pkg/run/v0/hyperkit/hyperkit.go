package hyperkit

import (
	"path/filepath"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	hyperkit "github.com/docker/infrakit/pkg/provider/hyperkit/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "hyperkit"

	// EnvDir is the environment variable for where the instance states are stored.
	EnvDir = "INFRAKIT_INSTANCE_HYPERKIT_DIR"

	// EnvDiscoveryHostPort is the environment variable to set to use for discovery of this plugin
	// by other infrakit components.  If infrakit is running in a container on Docker4Mac, use
	// 192.168.65.1.248645 (the default).  If infrakit is running as a process on a Mac, use
	// 127.0.0.1:24865 (loop back at the port this plugin is listening on).
	EnvDiscoveryHostPort = "INFRAKIT_INSTANCE_HYPERKIT_DISCOVERY_HOSTPORT"
)

var (
	log = logutil.New("module", "run/v0/hyperkit")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Dir is the directory for storing the VM state
	Dir string

	// HyperKitCmd is the hyperkit command to use
	HyperKitCmd string

	// VpnKitSock is the path to VpnKit unix domain socket
	VpnKitSock string

	// Listen is the port spec to listen on.
	Listen string

	// DiscoveryHostPort is the host:port used for other infrakit components to discover and connect to this plugin.
	// Use 192.168.65.1:24864 if running on Docker4Mac and if infrakit is running in a container.
	DiscoveryHostPort string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Listen:            ":24865",
	DiscoveryHostPort: local.Getenv(EnvDiscoveryHostPort, "192.168.65.1:24865"),
	Dir:               local.Getenv(EnvDir, filepath.Join(local.InfrakitHome(), "hyperkit-vms")),
	VpnKitSock:        "auto",
	HyperKitCmd:       "hyperkit",
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	transport.Listen = options.Listen
	transport.Advertise = options.DiscoveryHostPort

	impls = map[run.PluginCode]interface{}{
		run.Instance: hyperkit.NewPlugin(options.Dir, options.HyperKitCmd, options.VpnKitSock),
	}

	return
}
