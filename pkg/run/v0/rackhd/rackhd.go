package rackhd

import (
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	rackhd "github.com/docker/infrakit/pkg/provider/rackhd/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spiegela/gorackhd/monorail"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "rackhd"

	// EnvEndpoint is the endpoint where RackHD is listening
	EnvEndpoint = "INFRAKIT_INSTANCE_RACKHD_ENDPOINT"

	// EnvUsername is the username for RackHD
	EnvUsername = "INFRAKIT_INSTANCE_RACKHD_USERNAME"

	// EnvPassword is the password for RackHD
	EnvPassword = "INFRAKIT_INSTANCE_RACKHD_PASSWORD"
)

var (
	log = logutil.New("module", "run/v0/rackhd")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = rackhd.Options{
	Endpoint: local.Getenv(EnvEndpoint, "http://localhost:9090"),
	Username: local.Getenv(EnvUsername, "admin"),
	Password: local.Getenv(EnvPassword, "admin123"),
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	// Decode the options for settings like username, password, etc.
	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	// Name of the plugin to run as (the socket name)
	transport.Name = name

	// A map of rpc objects
	impls = map[run.PluginCode]interface{}{
		run.Instance: rackhd.NewInstancePlugin(monorail.New(options.Endpoint), options.Username, options.Password),
	}
	return
}
