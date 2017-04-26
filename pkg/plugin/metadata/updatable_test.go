package metadata

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/metadata"
	. "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

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
	u := &updatable{
		load: func() (*types.Any, error) {
			return types.AnyString(original), nil
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

	T(100).Infoln("current", current.String())
	T(100).Infoln("proposed", proposed.String())

	var v1, v2 interface{}

	require.NoError(t, current.Decode(&v1))
	require.NoError(t, proposed.Decode(&v2))

	require.Equal(t, 10., types.Get(types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"), v1))
	require.Equal(t, 20., types.Get(types.PathFromString("Groups/cattle/Properties/Properties/Allocations/Size"), v2))

}
