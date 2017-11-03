package manager

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

type key struct {
	Kind string
	Name string
}

type record struct {
	// Handler is the actual plugin used to process the input
	Handler plugin.Name

	// Spec is a copy of the spec
	Spec types.Spec
}

type persisted struct {
	Key    key
	Record record
}

type globalSpec struct {
	data  []persisted
	index map[key]record
}

func (g *globalSpec) visit(f func(key, record) error) error {
	for k, v := range g.index {
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (g *globalSpec) store(store store.Snapshot) error {
	data := []persisted{}
	for k, v := range g.index {
		data = append(data, persisted{Key: k, Record: v})
	}
	g.data = data
	return store.Save(g.data)
}

func (g *globalSpec) load(store store.Snapshot) error {
	g.data = []persisted{}
	err := store.Load(&g.data)
	if err != nil {
		return err
	}
	g.index = map[key]record{}
	for _, p := range g.data {
		g.index[p.Key] = p.Record
	}
	return nil
}

func (g *globalSpec) updateSpec(spec types.Spec, handler plugin.Name) {
	if g.index == nil {
		g.index = map[key]record{}
	}
	key := key{
		Kind: spec.Kind,
		Name: spec.Metadata.Name,
	}
	g.index[key] = record{
		Spec:    spec,
		Handler: handler,
	}
}

func keyFromGroupID(id group.ID) key {
	return key{
		// TODO(chungers) - the group value should be constant for the 'kind'.
		// Currently Kind is in the pkg/run/v0/group package and we can't have dependency on that because
		// the pkg/run is like main/ downstream from the core package here.  So we should refactor code a bit to
		// clean it up and make 'kind' more a top level concept.
		Kind: "group",
		Name: string(id),
	}
}

func (g *globalSpec) removeSpec(kind string, metadata types.Metadata) {
	if g.index == nil {
		g.index = map[key]record{}
	}
	delete(g.index, key{Kind: kind, Name: metadata.Name})
}

func (g *globalSpec) getSpec(kind string, metadata types.Metadata) (types.Spec, error) {
	if g.index == nil {
		g.index = map[key]record{}
	}
	r, has := g.index[key{Kind: kind, Name: metadata.Name}]
	if !has {
		return types.Spec{}, fmt.Errorf("not found %v %v", kind, metadata.Name)
	}
	return r.Spec, nil
}

func (g *globalSpec) removeGroup(id group.ID) {
	if g.index == nil {
		g.index = map[key]record{}
	}
	delete(g.index, keyFromGroupID(id))
}

func (g *globalSpec) getGroupSpec(id group.ID) (group.Spec, error) {
	if g.index == nil {
		g.index = map[key]record{}
	}

	gspec := group.Spec{
		ID: id,
	}
	record, has := g.index[keyFromGroupID(id)]
	if !has {
		return gspec, fmt.Errorf("not found %v", id)
	}
	gspec.Properties = record.Spec.Properties
	return gspec, nil
}

func (g *globalSpec) updateGroupSpec(gspec group.Spec, handler plugin.Name) {
	if g.index == nil {
		g.index = map[key]record{}
	}

	key := keyFromGroupID(gspec.ID)
	_, has := g.index[key]
	if !has {
		g.index[key] = record{
			Spec: types.Spec{
				Kind: "group",
				Metadata: types.Metadata{
					Name: string(gspec.ID),
				},
			},
			Handler: handler,
		}
	}
	record := g.index[key]
	record.Spec.Properties = gspec.Properties

	g.index[key] = record
}
