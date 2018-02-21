package internal

import (
	"fmt"
	"strings"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
)

var log = logutil.New("module", "rpc/internal")

const debugV = logutil.V(1000)

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

// Objects returns the objects exposed by this kind of RPC service
func (k *Keyed) Objects() []rpc.Object {
	m, err := k.listFunc()
	if err != nil {
		return nil
	}
	log.Debug("Objects", "map", m, "V", debugV)
	objs := []rpc.Object{}
	for key := range m {
		objs = append(objs, rpc.Object{Name: fmt.Sprintf("%v", key)})
	}
	return objs
}

// Do performs work calling the work function once the request resolves to an object
func (k *Keyed) Do(request Addressable, work func(resolved interface{}) error) error {
	resolved, err := k.Resolve(request)
	log.Debug("Do", "resolved", resolved, "err", err, "req", request, "V", debugV)
	if err != nil {
		return err
	}
	return work(resolved)
}

// Resolve resolves input (a request object for example) that implements the Addressable interface into a plugin
func (k *Keyed) Resolve(request Addressable) (interface{}, error) {
	to, err := request.Plugin()
	log.Debug("Resolve", "to", to, "err", err, "req", request, "V", debugV)
	if err != nil {
		return nil, err
	}
	return k.Keyed(to)
}

// Keyed performs a lookup of the object by plugin name
func (k *Keyed) Keyed(name plugin.Name) (interface{}, error) {
	m, err := k.listFunc()
	if err != nil {
		return nil, err
	}

	lookup, subtype := name.GetLookupAndType()
	log.Debug("Keyed", "m", m, "lookup", lookup, "subtype", subtype, "V", debugV)

	if (subtype == "" || lookup == ".") && len(m) == 1 {
		// this case we just match the default .
		for _, p := range m {
			return p, nil
		}
	}

	if subtype == "" && lookup != "." {
		// This is the case like vars but we have vars/aws... so we look for a '.' or top level plugin
		lookup = "."
	}

	if p, has := m[lookup]; has {
		return p, nil
	}

	// check to see if the subtype is actually a path.
	// shift by one
	shifted := subtype[strings.Index(subtype, "/")+1:]
	if p, has := m[shifted]; has {
		return p, nil
	}

	return nil, fmt.Errorf("not found: %v (key=%v)", name, subtype)
}
