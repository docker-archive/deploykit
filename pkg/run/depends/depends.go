package depends

import (
	"fmt"
	"sync"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "run/depends")

// ParseDependsFunc returns a list of dependencies of this spec.
type ParseDependsFunc func(types.Spec) (Runnables, error)

var (
	parsers = map[string]map[types.InterfaceSpec]ParseDependsFunc{}
	lock    = sync.RWMutex{}
)

// Register registers a helper for parsing for dependencies based on a key (e.g. 'group')
// and interface spec (Group/1.0)
func Register(key string, interfaceSpec types.InterfaceSpec, f ParseDependsFunc) {
	lock.Lock()
	defer lock.Unlock()

	if _, has := parsers[key]; !has {
		parsers[key] = map[types.InterfaceSpec]ParseDependsFunc{}
	}

	if _, has := parsers[key][interfaceSpec]; has {
		panic(fmt.Errorf("duplicate depdency parser for %v / %v", key, interfaceSpec))
	}
	parsers[key][interfaceSpec] = f
}

// Resolve returns the dependencies listed in the spec as well as inside the properties.
// InterfaceSpec is optional.  If nil, the first match by key (kind) is used.  If nothing is registered, returns nil
// and no error.  Error is returned for exceptions (eg. parsing, etc.)
func Resolve(spec types.Spec, key string, interfaceSpec *types.InterfaceSpec) (Runnables, error) {
	lock.RLock()
	defer lock.RUnlock()

	m, has := parsers[key]
	if !has {
		return nil, nil
	}
	if interfaceSpec == nil {
		for _, parse := range m {
			// First match
			return parse(spec)
		}
	}
	parse, has := m[*interfaceSpec]
	if !has {
		return nil, nil
	}
	return parse(spec)
}
