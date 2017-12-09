package depends

import (
	"fmt"
	"sort"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

// Runnable models an addressable object that can also be started.
type Runnable interface {
	plugin.Addressable
	// Options returns the options needed to start the plugin
	Options() *types.Any
	// Dependents return all the plugins this runnable depends on
	Dependents() (Runnables, error)
}

// Runnables represent a collection of Runnables
type Runnables []Runnable

// RunnableFrom creates a runnable from input name.  This is a simplification
// for cases where only a plugin name is used to reference another plugin.
func RunnableFrom(name plugin.Name) Runnable {
	kind := name.Lookup()
	return specQuery{
		Addressable: plugin.NewAddressable(kind, name, ""),
		spec: types.Spec{
			Kind: kind,
			Metadata: types.Metadata{
				Name: string(name),
			},
		},
	}
}

// AsRunnable returns the Runnable from a spec.
func AsRunnable(spec types.Spec) Runnable {
	return &specQuery{
		Addressable: plugin.AsAddressable(spec),
		spec:        spec,
	}
}

type specQuery struct {
	plugin.Addressable
	spec types.Spec
}

// Options returns the options
func (ps specQuery) Options() *types.Any {
	return ps.spec.Options
}

// Dependents returns the plugins depended on by this unit
func (ps specQuery) Dependents() (Runnables, error) {

	var interfaceSpec *types.InterfaceSpec
	if ps.spec.Version != "" {
		decoded := types.DecodeInterfaceSpec(ps.spec.Version)
		interfaceSpec = &decoded
	}
	dependentPlugins, err := Resolve(ps.spec, ps.Kind(), interfaceSpec)
	if err != nil {
		return nil, err
	}
	log.Debug("dependentPlugins", "depends", dependentPlugins, "spec", ps.spec, "kind", ps.Kind(), "intf", interfaceSpec)

	// join this with the dependencies already in the spec
	out := Runnables{}
	out = append(out, dependentPlugins...)

	for _, d := range ps.spec.Depends {
		out = append(out, AsRunnable(types.Spec{Kind: d.Kind, Metadata: types.Metadata{Name: d.Name}}))
	}

	log.Debug("dependents", "specQuery", ps, "result", out)
	return out, nil
}

// RunnablesFrom returns the Runnables from given slice of specs
func RunnablesFrom(specs []types.Spec) (Runnables, error) {

	key := func(addr plugin.Addressable) string {
		return fmt.Sprintf("%v::%v", addr.Kind(), addr.Plugin().Lookup())
	}

	keys := []string{}
	// keyed by kind and the specQuery
	all := map[string]Runnable{}
	for _, s := range specs {

		q := AsRunnable(s)
		all[key(q)] = q
		keys = append(keys, key(q))

		deps, err := q.Dependents()
		if err != nil {
			return nil, err
		}

		for _, d := range deps {
			all[key(d)] = d
			keys = append(keys, key(d))
		}
	}

	sort.Strings(keys)
	out := Runnables{}
	for _, k := range keys {
		out = append(out, all[k])
	}
	return out, nil
}
