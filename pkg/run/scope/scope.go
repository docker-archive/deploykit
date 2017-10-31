package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// Nil is no scope
var Nil = Scope{}

// Scope provides an environment in which the necessary plugins are available
// for doing a unit of work.  The scope can be local or remote, namespaced,
// depending on implementation.  The first implementation is to simply run
// a set of steps locally on a set of required plugins.  Because the scope
// provides the plugin lookup, it can control what plugins are available.
// This is good for programmatically control access of what a piece of code
// can interact with the system.
// Scope is named scope instead of 'context' because it's much heavier weight
// and involves lots of calls across process boundaries, yet it provides
// lookup and scoping of services based on some business logical and locality
// of code.
type Scope struct {

	// Plugins returns the plugin lookup
	Plugins func() discovery.Plugins

	// Instance is for looking up an instance plugin
	Instance InstanceResolver

	// Flavor is for lookup up a flavor plugin
	Flavor FlavorResolver

	// Metadata is for resolving metadata / path related queries
	Metadata MetadataResolver

	// TemplateEngine creates a template engine for use.
	TemplateEngine TemplateEngine
}

// Work is a unit of work that is executed in the scope of the plugins
// running. When work completes, the plugins are shutdown.
type Work func(Scope) error

// MetadataCall is a struct that has all the information needed to evaluate a template metadata function
type MetadataCall struct {
	Plugin metadata.Plugin
	Name   plugin.Name
	Key    types.Path
}

// TemplateEngine returns a configured template engine for this scope
type TemplateEngine func(url string, opts template.Options) (*template.Template, error)

// InstanceResolver resolves a string name for the plugin to instance plugin
type InstanceResolver func(n string) (instance.Plugin, error)

// FlavorResolver resolves a string name for the plugin to flavor plugin
type FlavorResolver func(n string) (flavor.Plugin, error)

// MetadataResolver is a function that can resolve a path to a callable to access metadata
type MetadataResolver func(p string) (*MetadataCall, error)

// DefaultScope returns the default scope
func DefaultScope(plugins func() discovery.Plugins) Scope {
	s := Scope{
		Plugins:  plugins,
		Metadata: DefaultMetadataResolver(plugins),
		Instance: DefaultInstanceResolver(plugins),
		Flavor:   DefaultFlavorResolver(plugins),
	}

	s.TemplateEngine = func(url string, opts template.Options) (*template.Template, error) {
		engine, err := template.NewTemplate(url, opts)
		if err != nil {
			return nil, err
		}

		return engine.WithFunctions(func() []template.Function {
			return []template.Function{
				{
					Name: "metadata",
					Description: []string{
						"Metadata function takes a path of the form \"plugin_name/path/to/data\"",
						"and calls GET on the plugin with the path \"path/to/data\".",
						"It's identical to the CLI command infrakit metadata cat ...",
					},
					Func: MetadataFunc(s),
				},
				{
					Name: "var",
					Func: func(name string, optional ...interface{}) (interface{}, error) {

						if len(optional) > 0 {
							return engine.Var(name, optional...), nil
						}

						v := engine.Ref(name)
						if v == nil {
							// If not resolved, try to interpret the path as a path for metadata...
							m, err := MetadataFunc(s)(name, optional...)
							if err != nil {
								return nil, err
							}
							v = m
						}

						if v == nil && engine.Options().MultiPass {
							return engine.DeferVar(name), nil
						}

						return v, nil
					},
				},
			}
		}), nil
	}

	return s
}
