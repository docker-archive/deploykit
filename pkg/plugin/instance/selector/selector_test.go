package selector

import (
	"sort"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestOptions(t *testing.T) {

	options := Options{
		Choice{
			Name: plugin.Name("us-west-2c"),
			Instances: []instance.LogicalID{
				instance.LogicalID("10.20.100.103"),
			},
			Affinity: types.AnyValueMust(map[string]interface{}{
				"weight": 40,
			}),
		},
		Choice{
			Name: plugin.Name("us-west-2a"),
			Instances: []instance.LogicalID{
				instance.LogicalID("10.20.100.101"),
			},
			Affinity: types.AnyValueMust(map[string]interface{}{
				"weight": 20,
			}),
		},
		Choice{
			Name: plugin.Name("us-west-2b"),
			Instances: []instance.LogicalID{
				instance.LogicalID("10.20.100.102"),
			},
			Affinity: types.AnyValueMust(map[string]interface{}{
				"weight": 10,
			}),
		},
	}

	sort.Sort(options)

	names := []string{}
	for _, c := range options {
		names = append(names, string(c.Name))
	}
	require.Equal(t, []string{
		"us-west-2a",
		"us-west-2b",
		"us-west-2c",
	}, names)

	require.True(t, options[0].HasLogicalID("10.20.100.101"))
	require.True(t, options[1].HasLogicalID("10.20.100.102"))
	require.True(t, options[2].HasLogicalID("10.20.100.103"))
	require.False(t, options[2].HasLogicalID("10.20.100.100"))
}
