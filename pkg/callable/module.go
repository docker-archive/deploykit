package callable

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

	m.lock.Lock()
	defer m.lock.Unlock()

	index := map[string]string{}

	if err := m.loadIndex(&index); err != nil {
		return err
	}

	callables := map[string]*Callable{}
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

		callables[name] = NewCallable(m.Scope, source.String(), params, m.Options)
	}

	m.Callables = callables
	return nil
}
