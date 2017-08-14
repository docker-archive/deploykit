package spread

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/spi/instance"
	instance_test "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestGetLabels(t *testing.T) {

	labels := map[string]string{
		"x": "x",
		"y": "y",
	}

	require.Equal(t, labels, getLabels(selector.Choice{
		Name: plugin.Name("zone1"),
		Affinity: types.AnyValueMust(
			AffinityArgs{Labels: labels},
		),
	}))

	require.Equal(t, map[string]string(nil), getLabels(selector.Choice{
		Name: plugin.Name("zone1"),
	}))
}

func TestSelectOne(t *testing.T) {

	labels1 := map[string]string{
		"x": "1",
		"y": "1",
	}
	labels2 := map[string]string{
		"x": "2",
		"y": "2",
	}

	choices := []selector.Choice{
		{
			Name: plugin.Name("zone1"),
			Affinity: types.AnyValueMust(
				AffinityArgs{Labels: labels1},
			),
		},
		{
			Name: plugin.Name("zone2"),
			Affinity: types.AnyValueMust(
				AffinityArgs{Labels: labels2},
			),
		},
	}

	called1, called2 := make(chan struct{}), make(chan struct{})
	m, err := SelectOne(instance.Spec{}, choices, func(c selector.Choice) instance.Plugin {
		switch c.Name {
		case choices[0].Name:
			return &instance_test.Plugin{
				DoDescribeInstances: func(l map[string]string, p bool) ([]instance.Description, error) {
					require.Equal(t, labels1, l)
					close(called1)
					return nil, nil
				},
			}
		case choices[1].Name:
			return &instance_test.Plugin{
				DoDescribeInstances: func(l map[string]string, p bool) ([]instance.Description, error) {
					require.Equal(t, labels2, l)
					close(called2)
					return nil, nil
				},
			}
		}
		return nil
	})

	<-called1
	<-called2

	require.NoError(t, err)
	require.Equal(t, choices[0], m)

	m, err = SelectOne(instance.Spec{}, choices, func(c selector.Choice) instance.Plugin {
		switch c.Name {
		case choices[0].Name:
			return &instance_test.Plugin{
				DoDescribeInstances: func(l map[string]string, p bool) ([]instance.Description, error) {
					return []instance.Description{
						{ID: instance.ID("1")},
						{ID: instance.ID("2")},
						{ID: instance.ID("3")},
					}, nil
				},
			}
		case choices[1].Name:
			return &instance_test.Plugin{
				DoDescribeInstances: func(l map[string]string, p bool) ([]instance.Description, error) {
					return []instance.Description{
						{ID: instance.ID("1")},
						{ID: instance.ID("2")},
					}, nil
				},
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, choices[1], m)

}
