package vars

import (
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "vars"

	// EnvTemplate is the env for the template to evaluate
	EnvTemplate = "INFRAKIT_VARS_TEMPLATE"
)

var (
	log = logutil.New("module", "run/v0/vars")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {

	// InitialTemplate is the url or literal (with str://) of the template to evaluate to initialize the values.
	// The template must evaluate to a map.  Slice is not supported
	InitialTemplate *string
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	InitialTemplate: ptr(local.Getenv(EnvTemplate, "")),
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	defer log.Info("Starting up vars plugin", "transport", transport, "impls", impls)

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	data := map[string]interface{}{}

	if options.InitialTemplate != nil {

		t, e := template.NewTemplate(*options.InitialTemplate, template.Options{})
		if e != nil {
			err = e
			return
		}

		view, e := t.Render(nil)
		if e != nil {
			err = e
			return
		}

		// now load this as a map
		if view != "" {
			if e := types.AnyString(view).Decode(&data); e != nil {
				any, e := types.AnyYAML([]byte(view))
				if e != nil {
					err = e
					return
				}
				err = any.Decode(&data)
				if err != nil {
					return
				}
			}
		}

		// Dump the vars from the map and add to the map...
		if globals, err := t.Globals(); err == nil {
			for k, v := range globals {
				types.Put(types.PathFromString(k), v, data)
			}
		}
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.MetadataUpdatable: metadata_plugin.NewUpdatablePlugin(metadata_plugin.NewPluginFromData(data),
			func(proposed *types.Any) error {
				return proposed.Decode(&data)
			},
		),
	}

	return
}
