package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "run/scope")

// Nil is no scope -- no infrakit services

type emptyscope int

func (s emptyscope) Find(name plugin.Name) (*plugin.Endpoint, error) {
	return nil, nil
}

func (s emptyscope) List() (map[string]*plugin.Endpoint, error) {
	return map[string]*plugin.Endpoint{}, nil
}

// Nil is a scope that points to nothing, no plugins can be accessed.
var Nil = DefaultScope(func() discovery.Plugins { return emptyscope(1) })

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
type Scope interface {

	// Plugins returns the plugin lookup
	Plugins() discovery.Plugins

	// Stack returns the stack that entails this scope
	Stack(n string) (stack.Interface, error)

	// Group is for looking up an group plugin
	Group(n string) (group.Plugin, error)

	// Controller returns the controller by name
	Controller(n string) (controller.Controller, error)

	// Instance is for looking up an instance plugin
	Instance(n string) (instance.Plugin, error)

	// Flavor is for lookup up a flavor plugin
	Flavor(n string) (flavor.Plugin, error)

	// L4 is for lookup up an L4 plugin
	L4(n string) (loadbalancer.L4, error)

	// Metadata is for resolving metadata / path related queries
	Metadata(p string) (*MetadataCall, error)

	// TemplateEngine creates a template engine for use.
	TemplateEngine(url string, opts template.Options) (*template.Template, error)
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

type fullScope func() discovery.Plugins

// Plugins implements plugin lookup
func (f fullScope) Plugins() discovery.Plugins {
	return f()
}

// TemplateEngine implmements factory for creating template engine
func (f fullScope) TemplateEngine(url string, opts template.Options) (*template.Template, error) {
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
				Func: MetadataFunc(f),
			},
			{
				Name: "var",
				Func: func(name string, optional ...interface{}) (interface{}, error) {

					if len(optional) > 0 {
						return engine.Var(name, optional...)
					}

					v := engine.Ref(name)
					if v == nil {
						// If not resolved, try to interpret the path as a path for metadata...
						m, err := MetadataFunc(f)(name, optional...)
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

// DefaultScope returns the default scope
func DefaultScope(plugins func() discovery.Plugins) Scope {
	return fullScope(plugins)
}
