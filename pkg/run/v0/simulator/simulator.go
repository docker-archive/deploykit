package simulator

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "simulator"

	// EnvStore is the enviornment variable to control the backend to use. e.g. 'mem', 'file'
	EnvStore = "mem"

	// EnvDir is the env for directory for file storage
	EnvDir = "INFRAKIT_SIMULATOR_DIR"

	// EnvInstanceNames is the env var to set for the instance spi type names (comma-delimited)
	EnvInstanceNames = "INFRAKIT_SIMULATOR_INSTANCE_NAMES"

	// EnvL4Name is the env var to set for the L4 name
	EnvL4Name = "INFRAKIT_SIMULATOR_L4_NAME"

	// StoreMem is the value for using memory store
	StoreMem = "mem"

	// StoreFile is the value for using file store
	StoreFile = "file"
)

var (
	log = logutil.New("module", "run/v0/simulator")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	Store         string
	Dir           string
	InstanceTypes []string
	L4HostName    string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Store:         run.GetEnv(EnvStore, "mem"),
	Dir:           run.GetEnv(EnvDir, filepath.Join(run.InfrakitHome(), "simulator")),
	InstanceTypes: strings.Split(run.GetEnv(EnvInstanceNames, "compute,net,disk"), ","),
	L4HostName:    "test.com",
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	os.MkdirAll(options.Dir, 0755)

	impls = map[run.PluginCode]interface{}{}

	instanceMap := map[string]instance.Plugin{}
	if len(options.InstanceTypes) > 0 {
		impls[run.Instance] = instanceMap
	}
	for _, n := range options.InstanceTypes {
		instanceMap[n] = NewInstance(n, options)
	}

	if options.L4HostName != "" {
		impls[run.L4] = NewL4(options.L4HostName, options)
	}

	transport.Name = name
	return
}
