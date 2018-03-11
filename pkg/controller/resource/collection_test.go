package resource

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/testing/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestOptionsWithDefaults(t *testing.T) {

	c, err := newCollection(
		scope.DefaultScope(),
		DefaultOptions)
	require.NoError(t, err)
	require.Equal(t, DefaultOptions.MinChannelBufferSize, c.(*collection).options.MinChannelBufferSize)
}

func TestKeyFromPath(t *testing.T) {

	{
		k, err := keyFromPath(types.PathFromString("mystack/resource/networking/net1/Properties/size"))
		require.NoError(t, err)
		require.Equal(t, "mystack", k)
	}
	{
		k, err := keyFromPath(types.PathFromString("./net1/Properties/size"))
		require.NoError(t, err)
		require.Equal(t, "net1", k)
	}

}

func TestGetByPath(t *testing.T) {

	m := map[string]instance.Description{
		"disk1": {
			ID: instance.ID("1"),
			Tags: map[string]string{
				"tag1": "1",
			},
			Properties: types.AnyValueMust(map[string]string{
				"size": "1TB",
			}),
		},
		"disk2": {
			ID: instance.ID("2"),
			Tags: map[string]string{
				"tag1": "2",
			},
			Properties: types.AnyValueMust(map[string]string{
				"size": "2TB",
			}),
		},
	}

	require.Equal(t, "1", types.Get(types.PathFromString(`disk1/Tags/tag1`), m))
	require.Equal(t, "2", types.Get(types.PathFromString(`disk2/Tags/tag1`), m))
	require.Equal(t, instance.ID("1"), types.Get(types.PathFromString(`disk1/ID`), m))
	require.Equal(t, "1TB", types.Get(types.PathFromString(`disk1/Properties/size`), m))
}

func TestProcessWatches(t *testing.T) {

	properties := testProperties(t)
	watch, watching := processWatches(properties)

	// check the file... count the number of occurrences
	require.Equal(t, 5, len(watch.watchers["az1-net1"]))
	require.Equal(t, 2, len(watch.watchers["az2-net2"]))
	require.Equal(t, 1, len(watching["az1-net2"]))

}
