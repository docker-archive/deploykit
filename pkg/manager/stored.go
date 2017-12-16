package manager

import (
	"fmt"
	"sort"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

type key struct {
	Kind string
	Name string
}

// Ideally we calculate a DAG from the entire set of specs, but
// for now, we just capture simple dependency ordering using a rank.
// If any kind isn't in this map then it's defaulted to 0
var kindRank = map[string]int{
	"group":      100,
	"ingress":    200,
	"enrollment": 300,
}

type keys []key

// Len is part of sort.Interface.
func (keys keys) Len() int {
	return len(keys)
}

// Swap is part of sort.Interface.
func (keys keys) Swap(i, j int) {
	keys[i], keys[j] = keys[j], keys[i]
}

// Less is part of sort.Interface.
func (keys keys) Less(i, j int) bool {
	ranki := kindRank[keys[i].Kind]
	rankj := kindRank[keys[j].Kind]
	diff := ranki - rankj
	if diff == 0 {
		return keys[i].Name < keys[j].Name
	}
	return diff < 0
}

type record struct {
	// Handler is the actual plugin used to process the input
	Handler plugin.Name

	// Spec is a copy of the spec
	Spec types.Spec
}

type entry struct {
	Key    key
	Record record
}

type globalSpec struct {
	data  []entry
	index map[key]record
}

// returns the keys in sorted order based on dependencies of kinds
func (g *globalSpec) orderedKeys() []key {
	all := keys{}
	for k := range g.index {
		all = append(all, k)
	}
	sort.Sort(all)
	return all
}

func (g *globalSpec) visit(f func(key, record) error) error {
	keys := g.orderedKeys()
	for _, k := range keys {
		v := g.index[k]
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (g *globalSpec) store(store store.Snapshot) error {
	data := []entry{}
	for k, v := range g.index {
		data = append(data, entry{Key: k, Record: v})
	}
	g.data = data
	return store.Save(g.data)
}

func (g *globalSpec) load(store store.Snapshot) error {
	g.data = []entry{}
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

func (g *globalSpec) toSpecs() types.Specs {
	specs := types.Specs{}
	for _, p := range g.data {
		specs = append(specs, p.Record.Spec)
	}
	sort.Sort(specs)
	return specs
}
