package tailer

import (
	"path/filepath"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/event/tailer"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "tailer"

	// EnvPath is the environment variable to set to tail when no additional configs are used.
	EnvPath = "INFRAKIT_TAILER_PATH"
)

var (
	log = logutil.New("module", "run/v0/tailer")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = tailer.Options{
	tailer.Rule{
		Path:      local.Getenv(EnvPath, filepath.Join(local.Getenv("PWD", ""), "test.log")),
		MustExist: false,
	},
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
	if len(options) == 0 {
		options = DefaultOptions
	}

	var events *tailer.Tailer
	events, err = tailer.NewPlugin(options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Metadata: metadata_plugin.NewPluginFromData(events.Data()),
		run.Event:    events,
	}
	onStop = func() { events.Stop() }
	return
}
