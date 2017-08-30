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

	called := make(chan []persisted, 1)

	store := fakeSnapshot{
		SaveFunc: func(o interface{}) error {
			if v, is := o.([]persisted); is {
				called <- v
				close(called)
			}
			return nil
		},
	}

	require.NoError(t, g.store(store))

	v := <-called
	require.EqualValues(t, []persisted{
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
