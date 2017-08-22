package internal

import (
	"fmt"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
)

var log = logutil.New("module", "rpc/internal")

const debugV = logutil.V(500)

// ServeKeyed returns a map containing keyed rpc objects
func ServeKeyed(listFunc func() (map[string]interface{}, error)) *Keyed {
	return &Keyed{listFunc: listFunc}
}

// ServeSingle returns a keyed that conforms to the net/rpc rpc call convention.
func ServeSingle(c interface{}) *Keyed {
	return ServeKeyed(func() (map[string]interface{}, error) {
		return map[string]interface{}{
			".": c,
		}, nil
	})
}

// Addressable is for RPC requests to implement so that the rpc handler can extract key from the RPC request.
type Addressable interface {
	Plugin() (plugin.Name, error)
}

// Keyed is a helper that manages multiple keyed rpc objects in a common namespace
type Keyed struct {
	listFunc func() (map[string]interface{}, error)
}

// Types returns the types exposed by this kind of RPC service
func (k *Keyed) Types() []string {
	m, err := k.listFunc()
	if err != nil {
		return nil
	}
	types := []string{}
	for key := range m {
		types = append(types, fmt.Sprintf("%v", key))
	}
	return types
}

func (k *Keyed) Do(request Addressable, work func(resolved interface{}) error) error {
	resolved, err := k.Resolve(request)
	if err != nil {
		return err
	}
	return work(resolved)
}

func (k *Keyed) Resolve(request Addressable) (interface{}, error) {
	to, err := request.Plugin()
	if err != nil {
		return nil, err
	}
	log.Debug("dispatching request", "to", to, "V", debugV)
	return k.Keyed(to)
}

func (k *Keyed) Keyed(name plugin.Name) (interface{}, error) {
	m, err := k.listFunc()
	if err != nil {
		return nil, err
	}
	l, subtype := name.GetLookupAndType()

	// Special case of single, unkeyed plugin object
	if l == "." {
		if len(m) == 1 {
			for _, v := range m {
				return v, nil // first value
			}
		}
	}

	if subtype == "." || subtype == "" {
		if len(m) == 1 {
			for _, v := range m {
				return v, nil // first value
			}
		}
	}

	if p, has := m[subtype]; has {
		return p, nil
	}
	return nil, fmt.Errorf("not found: %v (key=%v)", name, subtype)
}
