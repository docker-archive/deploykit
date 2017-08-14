package weighted

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestBiasesFromChoices(t *testing.T) {

	require.Equal(t, []int{10, 20, 30, 0}, biasesFrom(
		[]selector.Choice{
			{
				Name: plugin.Name("zone1"),
				Affinity: types.AnyValueMust(
					AffinityArgs{Weight: 10},
				),
			},
			{
				Name: plugin.Name("zone2"),
				Affinity: types.AnyValueMust(
					AffinityArgs{Weight: 20},
				),
			},
			{
				Name: plugin.Name("zone3"),
				Affinity: types.AnyValueMust(
					AffinityArgs{Weight: 30},
				),
			},
			{
				Name: plugin.Name("zone4"),
				Affinity: types.AnyValueMust(
					"bad input",
				),
			},
		},
	))

	require.Equal(t, []int{10, 20, 30}, biasesFrom(
		[]selector.Choice{
			{
				Name: plugin.Name("zone1"),
				Affinity: types.AnyValueMust(
					AffinityArgs{Weight: 10},
				),
			},
			{
				Name: plugin.Name("zone2"),
				Affinity: types.AnyValueMust(
					AffinityArgs{Weight: 20},
				),
			},
			{
				Name: plugin.Name("zone3"),
				Affinity: types.AnyValueMust(
					AffinityArgs{Weight: 30},
				),
			},
		},
	))

}

func TestRoll(t *testing.T) {

	bins := map[int]int{}
	biases := []int{
		20,
		80,
	}

	for i := 0; i < 100; i++ {
		require.True(t, roll(biases) < 100)
	}

	require.Equal(t, -1, bin(biases, 100))

	for i := 0; i < 100; i++ {
		bins[bin(biases, i)]++
	}
	require.Equal(t, 2, len(bins))
	require.Equal(t, 20, bins[0])
	require.Equal(t, 80, bins[1])

	bins = map[int]int{}
	for i := 0; i < 1000; i++ {
		bins[bin(biases, roll(biases))]++
	}
	require.Equal(t, 2, len(bins))
}

func TestSelectOne(t *testing.T) {

	bins := map[plugin.Name]int{}
	biases := []int{
		20,
		80,
	}

	choices := []selector.Choice{
		{
			Name: plugin.Name("zone1"),
			Affinity: types.AnyValueMust(
				AffinityArgs{Weight: uint(biases[0])},
			),
		},
		{
			Name: plugin.Name("zone2"),
			Affinity: types.AnyValueMust(
				AffinityArgs{Weight: uint(biases[1])},
			),
		},
	}
	for i := 0; i < 10000; i++ {
		m, err := SelectOne(instance.Spec{}, choices, nil)
		require.NoError(t, err)
		bins[m.Name]++
	}
	require.Equal(t, 2, len(bins))

	// p(zone2) = 4 * p(zone1), but we only have 10000 rolls. so this is here to avoid a flaky test
	require.True(t, bins[choices[1].Name]/bins[choices[0].Name] >= 3)
}
