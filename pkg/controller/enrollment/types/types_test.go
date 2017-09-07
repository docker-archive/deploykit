package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func mustSpec(s types.Spec, err error) types.Spec {
	if err != nil {
		panic(err)
	}
	return s
}

func specFromString(s string) (types.Spec, error) {
	v, err := types.AnyYAML([]byte(s))
	if err != nil {
		return types.Spec{}, err
	}
	spec := types.Spec{}
	err = v.Decode(&spec)
	return spec, err
}

func TestWriteProperties(t *testing.T) {
	p := Properties{
		List: (*ListSourceUnion)(types.AnyValueMust([]instance.Description{
			{ID: instance.ID("host1")},
			{ID: instance.ID("host2")},
		})),
		Instance: PluginSpec{
			Plugin:     plugin.Name("simulator/compute"),
			Properties: types.AnyValueMust("test"),
		},
	}

	buff, err := types.AnyValueMust(p).MarshalYAML()
	require.NoError(t, err)

	p2 := Properties{}
	err = types.AnyYAMLMust(buff).Decode(&p2)
	require.NoError(t, err)

	list1, err := p.List.InstanceDescriptions()
	require.NoError(t, err)

	list2, err := p2.List.InstanceDescriptions()
	require.NoError(t, err)

	require.EqualValues(t, list2, list1)
}

func TestParseProperties(t *testing.T) {

	spec := mustSpec(specFromString(`
kind: enrollment
metadata:
  name: nfs
properties:
  List:
    - ID: host1
    - ID: host2
    - ID: host3
    - ID: host4
  Instance:
    Plugin: us-east/nfs-authorizer
    Properties:
      Id: \{\{ .ID \}\}
`))

	p := Properties{}
	err := spec.Properties.Decode(&p)
	require.NoError(t, err)

	list, err := p.List.InstanceDescriptions()
	require.NoError(t, err)

	_, err = p.List.GroupPlugin()
	require.Error(t, err)

	require.EqualValues(t, []instance.Description{
		{ID: instance.ID("host1")},
		{ID: instance.ID("host2")},
		{ID: instance.ID("host3")},
		{ID: instance.ID("host4")},
	}, list)
}

func TestParsePropertiesWithGroup(t *testing.T) {

	spec := mustSpec(specFromString(`
kind: enrollment
metadata:
  name: nfs
properties:
  List: us-east/workers
  Instance:
    Plugin: us-east/nfs-authorizer
    Properties:
      Id: \{\{ .ID \}\}
`))

	p := Properties{}
	err := spec.Properties.Decode(&p)
	require.NoError(t, err)

	_, err = p.List.InstanceDescriptions()
	require.Error(t, err)

	g, err := p.List.GroupPlugin()
	require.NoError(t, err)
	require.Equal(t, plugin.Name("us-east/workers"), g)
}
