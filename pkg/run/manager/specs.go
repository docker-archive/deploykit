package manager

import (
	"github.com/docker/infrakit/pkg/launch/inproc"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/types"
)

type specQuery struct {
	types.Spec
}

// Kind returns the kind to use for launching.  It's assumed these map to something in the launch Rules.
func (ps specQuery) Kind() string {
	lookup, _ := ps.Plugin().GetLookupAndType()
	return lookup
}

// Plugin derives a plugin name from the record
func (ps specQuery) Plugin() plugin.Name {
	pn := plugin.Name(ps.Spec.Kind)
	if lookup, sub := pn.GetLookupAndType(); sub == "" {
		return plugin.NameFrom(lookup, ps.Spec.Metadata.Name)
	}
	return pn
}

// Options returns the options
func (ps specQuery) Options() *types.Any {
	return ps.Spec.Options
}

// Dependents returns the plugins depended on by this unit
func (ps specQuery) Dependents() (specQueries, error) {

	var interfaceSpec *types.InterfaceSpec
	if ps.Spec.Version != "" {
		decoded := types.DecodeInterfaceSpec(ps.Spec.Version)
		interfaceSpec = &decoded
	}
	dependentPlugins, err := depends.Resolve(ps.Spec, ps.Kind(), interfaceSpec)
	if err != nil {
		return nil, err
	}
	// join this with the dependencies already in the spec
	out := specQueries{}
	for _, d := range dependentPlugins {
		out = append(out, specQuery{types.Spec{Kind: d.String(), Metadata: types.Metadata{Name: d.String()}}})
	}
	for _, d := range ps.Depends {
		out = append(out, specQuery{types.Spec{Kind: d.Kind, Metadata: types.Metadata{Name: d.Name}}})
	}

	log.Debug("dependents", "specQuery", ps, "result", out)
	return out, nil
}

type specQueries []specQuery

func startupInstructions(specs []types.Spec) (specQueries, error) {
	// keyed by kind and the specQuery
	all := map[string]specQuery{}
	for _, s := range specs {
		q := specQuery{s}
		all[q.Kind()] = q

		deps, err := q.Dependents()
		if err != nil {
			return nil, err
		}

		for _, d := range deps {
			// last win -- check for configs?  atm just focus on referenced objects
			all[d.Kind()] = d
		}
	}

	log.Debug("StartUpInstructions", "all", all)
	out := specQueries{}
	for _, s := range all {
		out = append(out, s)
	}
	return out, nil
}

func (m *Manager) validate(all specQueries) error {
	for _, s := range all {
		log.Debug("Validate", "kind", s.Kind(), "name", s.Plugin(), "options", s.Options())
	}
	return nil
}

// StartPluginsFromSpecs starts up the plugins referenced in the specs
func (m *Manager) StartPluginsFromSpecs(specs []types.Spec, onError func(error) bool) error {

	instructions, err := startupInstructions(specs)
	if err != nil {
		return err
	}
	if err := m.validate(instructions); err != nil {
		if !onError(err) {
			return err
		}
	}

	for _, q := range instructions {

		log.Debug("Launching", "exec", inproc.ExecName, "kind", q.Kind(), "name", q.Plugin(), "options", q.Options())

		if err := m.Launch(inproc.ExecName, q.Kind(), q.Plugin(), q.Options()); err != nil {
			if !onError(err) {
				return err
			}
		}
	}
	return nil
}
