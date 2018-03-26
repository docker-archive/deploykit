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

func TestProcessProvisionWatches(t *testing.T) {

	properties := testProperties(t)
	provisionWatch, provisionWatching := processProvisionWatches(properties)

	// check the file... count the number of occurrences
	require.Equal(t, 5, len(provisionWatch.watchers["az1-net1"]))
	require.Equal(t, 2, len(provisionWatch.watchers["az2-net2"]))
	require.Equal(t, 1, len(provisionWatching["az1-net2"]))
}

func TestProcessDestroyWatches(t *testing.T) {

	properties := testProperties(t)
	destroyWatch, destroyWatching := processDestroyWatches(properties)

	// check the file... count the number of occurrences
	require.Equal(t, 0, len(destroyWatch.watchers["az1-net1"]))
	require.Equal(t, 1, len(destroyWatch.watchers["az2-net2"]))
	require.Equal(t, 1, len(destroyWatch.watchers["az1-net2"]))
	require.Equal(t, 2, len(destroyWatching["az1-net2"]))
}

func TestProcessDestroyWatches2(t *testing.T) {

	buff := []byte(`
kind: resource
metadata:
  name: resources
options:
  WaitBeforeProvision: 100
properties:
  A:
    plugin: az1/net
    Properties:
      prop1: A-1
      prop2: A-2
  B:
    plugin: az1/net
    Properties:
      wait: "@depend('A/ID')@"
      prop1: B-1
      prop2: B-2
  C:
    plugin: az1/net
    Properties:
      wait1: "@depend('A/ID')@"
      wait2: "@depend('B/ID')@"
      wait3: "@depend('post-provision:A/data/joinToken')@"
      prop1: C-1
      prop2: C-2
  D:
    plugin: az1/net
    init: |
      cluster join --token @depend('post-provision:A/data/joinToken')@ @depend('A/Properties/address')@
    Properties:
      wait1: "@depend('A/ID')@"
      wait2: "@depend('B/ID')@"
      wait3: "@depend('C/ID')@"
      prop1: D-1
      prop2: D-2
`)

	var spec types.Spec
	err := types.Decode(buff, &spec)
	require.NoError(t, err)

	properties := DefaultProperties
	err = spec.Properties.Decode(&properties)
	require.NoError(t, err)

	// Provisioning order ==> B depends on A, so A must run before B
	provisionWatch, provisionWatching := processProvisionWatches(properties)
	require.Equal(t, 4, len(provisionWatch.watchers["A"]))                // To provision, 4 expressions depends on A
	require.Equal(t, 0, len(provisionWatching["A"]))                      // A depends on 0
	require.Equal(t, 2, len(provisionWatch.watchers["post-provision:A"])) // C and D
	require.Equal(t, 0, len(provisionWatching["post-provision:A"]))       // Technically on A but treat it as distinct
	require.Equal(t, 2, len(provisionWatch.watchers["B"]))                // 2 depends on B
	require.Equal(t, 1, len(provisionWatching["B"]))                      // B depends on 1 (A)
	require.Equal(t, 1, len(provisionWatch.watchers["C"]))                // D
	require.Equal(t, 3, len(provisionWatching["C"]))                      // A, post-provision:A and B
	require.Equal(t, 0, len(provisionWatch.watchers["D"]))                // 0 watchers on D
	require.Equal(t, 5, len(provisionWatching["D"]))                      // A, B, C

	// Destroy order ==> in reverse
	destroyWatch, destroyWatching := processDestroyWatches(properties)
	require.Equal(t, 0, len(destroyWatch.watchers["A"])) // 0 waits for A to be destroyed before it can be destroyed.
	require.Equal(t, 4, len(destroyWatching["A"]))       // 5 references A so A is watching on 5
	require.Equal(t, 1, len(destroyWatch.watchers["B"])) // A waits on B
	require.Equal(t, 2, len(destroyWatching["B"]))       // C and D
	require.Equal(t, 2, len(destroyWatch.watchers["C"])) // A and B
	require.Equal(t, 1, len(destroyWatching["C"]))       // D
	require.Equal(t, 4, len(destroyWatch.watchers["D"])) // A, B, C, and post-provision:A
	require.Equal(t, 0, len(destroyWatching["D"]))       // 0

}
