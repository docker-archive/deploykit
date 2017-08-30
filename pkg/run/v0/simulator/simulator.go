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
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "simulator"

	// EnvStore is the enviornment variable to control the backend to use. e.g. 'mem', 'file'
	EnvStore = "mem"

	// EnvDir is the env for directory for file storage
	EnvDir = "INFRAKIT_SIMULATOR_DIR"

	// EnvInstanceTypes is the env var to set for the instance spi type names (comma-delimited)
	EnvInstanceTypes = "INFRAKIT_SIMULATOR_INSTANCE_TYPES"

	// EnvL4Names is the env var to set for the L4 name
	EnvL4Names = "INFRAKIT_SIMULATOR_L4_NAMES"

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
	L4Names       []string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Store:         run.GetEnv(EnvStore, "mem"),
	Dir:           run.GetEnv(EnvDir, filepath.Join(run.InfrakitHome(), "simulator")),
	InstanceTypes: strings.Split(run.GetEnv(EnvInstanceTypes, "compute,net,disk"), ","),
	L4Names:       strings.Split(run.GetEnv(EnvL4Names, "lb1,lb2,lb3"), ","),
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

	l4Map := map[string]loadbalancer.L4{}
	if len(options.L4Names) > 0 {
		impls[run.L4] = func() (map[string]loadbalancer.L4, error) { return l4Map, nil }
	}
	for _, n := range options.L4Names {
		l4Map[n] = NewL4(n, options)
	}

	transport.Name = name
	return
}
