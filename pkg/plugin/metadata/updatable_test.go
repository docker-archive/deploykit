package metadata

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/metadata"
	. "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestUpdatable(t *testing.T) {

	data := map[string]interface{}{
		"a": "A",
		"b": struct{ C string }{C: "C"},
		"c": "d",
	}
	readonly := NewPluginFromData(data)

	var store *types.Any
	commit := func(proposed *types.Any) error {
		store = proposed
		return nil
	}

	u := NewUpdatablePlugin(readonly, commit)

	buff, err := u.Get(types.PathFromString("a"))
	require.NoError(t, err)
	require.Equal(t, types.AnyString(`"A"`), buff)

	buff, err = u.Get(types.PathFromString("b/C"))
	require.NoError(t, err)
	require.Equal(t, types.AnyString(`"C"`), buff)

	before, proposed, cas, err := u.Changes([]metadata.Change{
		{
			Path:  types.PathFromString("e"),
			Value: types.AnyValueMust(10),
		},
		{
			Path:  types.PathFromString("b/C"),
			Value: types.AnyValueMust("X"),
		},
	})

	require.Equal(t, types.AnyValueMust(data), before)
	require.True(t, len(cas) > 0)
	require.Equal(t, types.AnyValueMust(map[string]interface{}{
		"a": "A",
		"b": struct{ C string }{C: "X"},
		"c": "d",
		"e": 10,
	}), proposed)

}

func TestUpdatableOverlayChanges(t *testing.T) {

	original := `
{
   "Groups" : {
      "cattle" : {
         "Properties" : {
            "Properties" : {
               "Allocations" : {
                  "Size" : 10
               },
               "Init" : "init"
            }
         }
      },
      "pets" : {
         "Properties" : {
            "Properties" : {
               "Allocations" : {
                  "Size" : 100
               },
               "Init" : "pets-init"
            }
         }
      }
   }
}
`
	commitChan := make(chan *types.Any, 1)

	data := map[string]interface{}{}
	require.NoError(t, types.AnyString(original).Decode(&data))

	u := &updatable{
		Plugin: NewPluginFromData(data),
		commit: func(proposed *types.Any) error {
			commitChan <- proposed
			return nil
		},
	}

	changes := []metadata.Change{
		{
			Path:  types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"),
			Value: types.AnyValueMust(20),
		},
	}

	current, proposed, cas, err := u.Changes(changes)
	require.NoError(t, err)

	T(100).Infoln("cas=", cas)
	require.NotEqual(t, "", cas)
	require.Equal(t, types.Fingerprint(current, proposed), cas)

	T(100).Infoln("current", current.String())
	T(100).Infoln("proposed", proposed.String())

	var v1, v2 interface{}

	require.NoError(t, current.Decode(&v1))
	require.NoError(t, proposed.Decode(&v2))

	require.Equal(t, 10., types.Get(types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"), v1))
	require.Equal(t, 20., types.Get(types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"), v2))

	require.NoError(t, u.Commit(proposed, cas))
	require.Equal(t, proposed, <-commitChan)

}

func TestUpdatableOverlayChangesResetAndCommit(t *testing.T) {

	original := `
{
   "Groups" : {
      "cattle" : {
         "Properties" : {
            "Properties" : {
               "Allocations" : {
                  "Size" : 10
               },
               "Init" : "init"
            }
         }
      },
      "pets" : {
         "Properties" : {
            "Properties" : {
               "Allocations" : {
                  "Size" : 100
               },
               "Init" : "pets-init"
            }
         }
      }
   }
}
`
	commitChan := make(chan *types.Any, 1)

	data := map[string]interface{}{}
	require.NoError(t, types.AnyString(original).Decode(&data))

	u := &updatable{
		Plugin: NewPluginFromData(data),
		commit: func(proposed *types.Any) error {
			commitChan <- proposed
			return nil
		},
	}

	changes := []metadata.Change{
		{
			Path:  types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"),
			Value: types.AnyValueMust(20),
		},
	}

	current, proposed, cas, err := u.Changes(changes)
	require.NoError(t, err)
	require.NoError(t, u.Commit(proposed, cas))
	require.Equal(t, proposed, <-commitChan)

	require.True(t, current.String() != proposed.String())

	// now reset
	var reset interface{}
	require.NoError(t, types.AnyString(original).Decode(&reset))

	_, proposed2, cas2, err := u.Changes([]metadata.Change{
		{
			Path:  types.Dot,
			Value: types.AnyValueMust(reset),
		},
	})
	require.NoError(t, err)
	require.NoError(t, u.Commit(proposed2, cas2))

	sz, err := u.Get(types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"))
	require.NoError(t, err)
	require.Equal(t, types.AnyValueMust(10.), sz)

	current3, proposed3, _, err := u.Changes([]metadata.Change{})
	require.NoError(t, err)
	require.Equal(t, current3.Bytes(), proposed3.Bytes())
	require.Equal(t, types.AnyValueMust(reset).Bytes(), current3.Bytes())
}
