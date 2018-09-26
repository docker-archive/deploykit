package callable // import "github.com/docker/infrakit/pkg/callable"

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"sync"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// Module is a collection of callable specified at a given index url
type Module struct {
	Scope          scope.Scope
	IndexURL       string
	Options        Options
	ParametersFunc func() backend.Parameters
	Callables      map[string]*Callable
	Modules        map[string]*Module

	lock sync.RWMutex
}

func (m *Module) loadIndex(model interface{}) error {
	// an index at the path specified is rendered and parsed into the provided
	// model.
	t, err := template.NewTemplate(m.IndexURL, m.Options.TemplateOptions)
	if err != nil {
		return err
	}
	buff, err := t.Render(nil)
	if err != nil {
		return err
	}
	return types.Decode([]byte(buff), model)
}

// GetCallable gets the loaded callable in this module by name.
func (m *Module) GetCallable(name string) (*Callable, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	c := m.Callables[name]
	if c == nil {
		return nil, fmt.Errorf("not found %v", name)
	}

	var parameters backend.Parameters
	if m.ParametersFunc != nil {
		parameters = m.ParametersFunc()
	} else {
		parameters = &Parameters{}
	}
	return c.Clone(parameters)
}

// GetModule gets the loaded sub module by name
func (m *Module) GetModule(name string) (*Module, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	c, has := m.Modules[name]
	if !has {
		return nil, fmt.Errorf("not found %v", name)
	}
	return c, nil
}

// Find takes a path and returns a nested callable, if found, or error.
func (m *Module) Find(path []string) (*Callable, error) {
	switch len(path) {
	case 0:
		return nil, fmt.Errorf("no path")
	case 1:
		return m.GetCallable(path[0])
	default:
		mm, err := m.GetModule(path[0])
		if err != nil {
			return nil, err
		}
		return mm.Find(path[1:])
	}
}

// List returns a list of callable names
func (m *Module) List() []string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if len(m.Callables) == 0 {
		return nil
	}

	names := []string{}
	for k := range m.Callables {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Load loads and initializes the module from the source
func (m *Module) Load() error {

	index := map[string]string{}
	if err := m.loadIndex(&index); err != nil {
		return err
	}

	callables := map[string]*Callable{}
	modules := map[string]*Module{}
	for name, sub := range index {

		fullURL := path.Join(path.Dir(m.IndexURL), sub)

		source, err := url.Parse(fullURL)
		if err != nil {
			log.Error("Cannot get path", "name", name, "index", m.IndexURL)
			continue
		}

		var params backend.Parameters
		if m.ParametersFunc != nil {
			params = m.ParametersFunc()
		} else {
			params = &Parameters{} // default in-memory map
		}

		// Because a URL like https://pb.com/p/foo can point to just a directory
		// there is no easy way to tell if a callable is in fact module (directory)
		// So here we do the expensive operation of defining all the params and check for any errors.
		// If an error is seen, then it's not included as a searchable callable.
		callable := NewCallable(m.Scope, source.String(), params, m.Options)
		if err := callable.DefineParameters(); err == nil {
			callables[name] = callable
		} else {
			// check for moudule.  A module has `index.yml` inside a directory
			sub := Module{
				Scope:          m.Scope,
				IndexURL:       path.Join(source.String(), "index.yml"),
				Options:        m.Options,
				ParametersFunc: m.ParametersFunc,
				Callables:      map[string]*Callable{},
				Modules:        map[string]*Module{},
			} // shallow copy
			if err := sub.Load(); err == nil {
				modules[name] = &sub
			}
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.Callables = callables
	m.Modules = modules
	return nil
}
