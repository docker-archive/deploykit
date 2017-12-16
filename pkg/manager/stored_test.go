package manager

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

type fakeSnapshot struct {
	SaveFunc func(interface{}) error
	LoadFunc func(interface{}) error
}

func (f fakeSnapshot) Close() error {
	return nil
}

func (f fakeSnapshot) Save(obj interface{}) error {
	return f.SaveFunc(obj)
}

func (f fakeSnapshot) Load(obj interface{}) error {
	return f.LoadFunc(obj)
}

func TestSortRecords(t *testing.T) {

	rank := kindRank["simulator"]
	require.Equal(t, 0, rank)

	require.True(t, kindRank["group"] < kindRank["ingress"])

	g := globalSpec{}

	s0 := types.Spec{
		Kind: "group",
		Metadata: types.Metadata{
			Name: "workers",
		},
		Properties: types.AnyValueMust(map[string]interface{}{"size": 10}),
	}
	g.updateSpec(s0, plugin.Name("group-stateless"))

	s1 := types.Spec{
		Kind: "group",
		Metadata: types.Metadata{
			Name: "managers",
		},
		Properties: types.AnyValueMust(map[string]interface{}{"size": 3}),
	}
	g.updateSpec(s1, plugin.Name("group-stateless"))

	s2 := types.Spec{
		Kind: "enrollment",
		Metadata: types.Metadata{
			Name: "nfs/workers",
		},
		Properties: types.AnyValueMust(map[string]interface{}{"shared": true}),
	}
	g.updateSpec(s2, plugin.Name("enrollment"))

	s3 := types.Spec{
		Kind: "ingress",
		Metadata: types.Metadata{
			Name: "workers/ingress",
		},
		Properties: types.AnyValueMust(map[string]interface{}{"routes": 10}),
	}
	g.updateSpec(s3, plugin.Name("ingress"))

	ordered := []key{
		{
			Kind: s1.Kind,
			Name: s1.Metadata.Name,
		},
		{
			Kind: s0.Kind,
			Name: s0.Metadata.Name,
		},
		{
			Kind: s3.Kind,
			Name: s3.Metadata.Name,
		},
		{
			Kind: s2.Kind,
			Name: s2.Metadata.Name,
		},
	}
	require.Equal(t, ordered, g.orderedKeys())

	visited := []key{}
	err := g.visit(func(k key, r record) error {
		visited = append(visited, k)
		return nil
	})
	require.Equal(t, ordered, visited)
	require.NoError(t, err)
}

func TestStoredRecords(t *testing.T) {

	g := globalSpec{}

	s := types.Spec{
		Kind: "group",
		Metadata: types.Metadata{
			Name: "workers",
		},
		Properties: types.AnyValueMust(map[string]interface{}{"a": 1, "b": 2}),
	}
	g.updateSpec(s, plugin.Name("group-stateless"))

	require.EqualValues(t, s, g.index[key{Kind: "group", Name: "workers"}].Spec)
	require.EqualValues(t, plugin.Name("group-stateless"), g.index[key{Kind: "group", Name: "workers"}].Handler)
	require.EqualValues(t, key{Kind: "group", Name: "workers"}, keyFromGroupID(group.ID("workers")))

	gspec, err := g.getGroupSpec(group.ID("workers"))
	require.NoError(t, err)

	require.EqualValues(t, group.Spec{ID: group.ID("workers"), Properties: s.Properties}, gspec)

	g.removeGroup(group.ID("workers"))
	g.removeGroup(group.ID("bad"))

	_, err = g.getGroupSpec(group.ID("workers"))
	require.Error(t, err)

	_, err = g.getGroupSpec(group.ID("bad"))
	require.Error(t, err)

	g.updateGroupSpec(group.Spec{ID: group.ID("workers"), Properties: s.Properties}, plugin.Name("group-stateless"))

	spec, err := g.getSpec("group", types.Metadata{Name: "workers"})
	require.NoError(t, err)
	require.EqualValues(t, types.Spec{
		Kind:       "group",
		Metadata:   types.Metadata{Name: "workers"},
		Properties: s.Properties,
	}, spec)

	g.removeSpec("group", types.Metadata{Name: "workers"})
	_, err = g.getSpec("group", types.Metadata{Name: "workers"})
	require.Error(t, err)

	g.updateGroupSpec(group.Spec{ID: group.ID("managers"), Properties: s.Properties}, plugin.Name("group-stateless"))

	called := make(chan []entry, 1)

	store := fakeSnapshot{
		SaveFunc: func(o interface{}) error {
			if v, is := o.([]entry); is {
				called <- v
				close(called)
			}
			return nil
		},
	}

	require.NoError(t, g.store(store))

	v := <-called
	require.EqualValues(t, []entry{
		{
			Key: key{Kind: "group", Name: "managers"},
			Record: record{Handler: plugin.Name("group-stateless"),
				Spec: types.Spec{
					Kind:       "group",
					Metadata:   types.Metadata{Name: "managers"},
					Properties: s.Properties,
				},
			},
		},
	}, v)

	disk := make(chan []byte, 1)
	store.SaveFunc = func(o interface{}) error {
		buff, err := types.AnyValueMust(o).MarshalJSON()
		if err != nil {
			return err
		}
		disk <- buff
		close(disk)
		return nil
	}

	store.LoadFunc = func(o interface{}) error {
		buff := <-disk
		return types.AnyBytes(buff).Decode(o)
	}

	require.NoError(t, g.store(store))

	g2 := globalSpec{}
	require.NoError(t, g2.load(store))

	require.EqualValues(t, record{Handler: plugin.Name("group-stateless"),
		Spec: types.Spec{
			Kind:       "group",
			Metadata:   types.Metadata{Name: "managers"},
			Properties: s.Properties,
		},
	}, g2.index[key{Kind: "group", Name: "managers"}])
}
