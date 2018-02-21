package scope

import (
	"fmt"
	"net/url"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/template"
)

// FakeLeader returns a fake leadership func
func FakeLeader(v bool) func() stack.Leadership {
	return func() stack.Leadership { return fakeLeaderT(v) }
}

type fakeLeaderT bool

func (f fakeLeaderT) IsLeader() (bool, error) {
	return bool(f), nil
}

func (f fakeLeaderT) LeaderLocation() (*url.URL, error) {
	return nil, nil
}

type fakePlugins map[string]*plugin.Endpoint

// Find implements discovery.Plugins
func (f fakePlugins) Find(name plugin.Name) (*plugin.Endpoint, error) {
	if f == nil {
		return nil, fmt.Errorf("not found")
	}

	lookup, _ := name.GetLookupAndType()
	if v, has := f[lookup]; has {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}

// List implements discovery.Plugins
func (f fakePlugins) List() (map[string]*plugin.Endpoint, error) {
	return (map[string]*plugin.Endpoint)(f), nil
}

// FakeScope returns a fake Scope with given endpoints
func FakeScope(endpoints map[string]*plugin.Endpoint) scope.Scope {
	return scope.DefaultScope(func() discovery.Plugins {
		return fakePlugins(endpoints)
	})
}

// DefaultScope returns a default scope but customizable for different plugin lookups
func DefaultScope() *Scope {
	f, err := local.NewPluginDiscovery()
	if err != nil {
		panic(err)
	}
	return &Scope{
		Scope: scope.DefaultScope(func() discovery.Plugins { return f }),
	}
}

// Scope is the testing scope for looking up components
type Scope struct {
	scope.Scope

	// ResolvePlugins returns the plugin lookup
	ResolvePlugins func() discovery.Plugins

	// ResolveStack returns the stack that entails this scope
	ResolveStack func(n string) (stack.Interface, error)

	// ResolveGroup is for looking up an group plugin
	ResolveGroup func(n string) (group.Plugin, error)

	// ResolveController returns the controller by name
	ResolveController func(n string) (controller.Controller, error)

	// ResolveInstance is for looking up an instance plugin
	ResolveInstance func(n string) (instance.Plugin, error)

	// ResolveFlavor is for lookup up a flavor plugin
	ResolveFlavor func(n string) (flavor.Plugin, error)

	// ResolveL4 is for lookup up an L4 plugin
	ResolveL4 func(n string) (loadbalancer.L4, error)

	// ResolveMetadata is for resolving metadata / path related queries
	ResolveMetadata func(p string) (*scope.MetadataCall, error)

	// ResolveTemplateEngine creates a template engine for use.
	ResolveTemplateEngine func(url string, opts template.Options) (*template.Template, error)
}

// Plugins returns the plugin lookup
func (s *Scope) Plugins() discovery.Plugins {
	if s.ResolvePlugins != nil {
		return s.ResolvePlugins()
	}
	return s.Scope.Plugins()
}

// Stack returns the stack that entails this scope
func (s *Scope) Stack(name string) (stack.Interface, error) {
	if s.ResolveStack != nil {
		return s.ResolveStack(name)
	}
	return s.Scope.Stack(name)
}

// Group is for looking up an group plugin
func (s *Scope) Group(name string) (group.Plugin, error) {
	if s.ResolveGroup != nil {
		return s.ResolveGroup(name)
	}
	return s.Scope.Group(name)
}

// Controller returns the controller by name
func (s *Scope) Controller(name string) (controller.Controller, error) {
	if s.ResolveController != nil {
		return s.ResolveController(name)
	}
	return s.Scope.Controller(name)
}

// Instance is for looking up an instance plugin
func (s *Scope) Instance(name string) (instance.Plugin, error) {
	if s.ResolveInstance != nil {
		return s.ResolveInstance(name)
	}
	return s.Scope.Instance(name)
}

// Flavor is for lookup up a flavor plugin
func (s *Scope) Flavor(name string) (flavor.Plugin, error) {
	if s.ResolveFlavor != nil {
		return s.ResolveFlavor(name)
	}
	return s.Scope.Flavor(name)
}

// L4 is for lookup up an L4 plugin
func (s *Scope) L4(name string) (loadbalancer.L4, error) {
	if s.ResolveL4 != nil {
		return s.ResolveL4(name)
	}
	return s.Scope.L4(name)
}

// Metadata is for resolving metadata / path related queries
func (s *Scope) Metadata(path string) (*scope.MetadataCall, error) {
	if s.ResolveMetadata != nil {
		return s.ResolveMetadata(path)
	}
	return s.Scope.Metadata(path)
}

// TemplateEngine creates a template engine for use.
func (s *Scope) TemplateEngine(url string, opts template.Options) (*template.Template, error) {
	if s.ResolveTemplateEngine != nil {
		return s.ResolveTemplateEngine(url, opts)
	}
	return s.Scope.TemplateEngine(url, opts)
}
