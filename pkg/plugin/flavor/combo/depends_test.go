package combo

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestDependencies(t *testing.T) {

	spec := types.Spec{
		Kind: "combo",
		Metadata: types.Metadata{
			Name: "managers",
		},
		Properties: types.AnyValueMust(
			Spec{
				{
					Plugin: plugin.Name("swarm/manager"),
					Properties: types.AnyValueMust(
						map[string]interface{}{
							"docker": "unix:///var/run/docker.sock",
						},
					),
				},
				{
					Plugin: plugin.Name("kubernetes/manager"),
					Properties: types.AnyValueMust(
						map[string]interface{}{
							"addOns": "weave",
						},
					),
				},
			},
		),
	}

	runnables, err := ResolveDependencies(spec)
	require.NoError(t, err)
	require.Equal(t, "swarm", runnables[0].Plugin().Lookup())
	require.Equal(t, "kubernetes", runnables[1].Plugin().Lookup())
}
