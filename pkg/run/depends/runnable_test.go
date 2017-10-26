package depends

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func mustSpec(s types.Spec, err error) types.Spec {
	if err != nil {
		panic(err)
	}
	return s
}

func mustSpecs(s []types.Spec, err error) []types.Spec {
	if err != nil {
		panic(err)
	}
	return s
}

func TestRunnable(t *testing.T) {
	v := types.DecodeInterfaceSpec("Test/0.1")
	Register("TestRunnable", v, func(spec types.Spec) (Runnables, error) {
		return Runnables{AsRunnable(spec)}, nil
	})

	runnable := AsRunnable(mustSpec(types.SpecFromString(`
kind: group
metadata:
  name: workers
properties:
  max: 100
  min: 10
options:
  poll: 10
`)))

	require.Equal(t, "group", runnable.Kind())
	require.Equal(t, plugin.Name("group/workers"), runnable.Plugin())
	require.Equal(t, "workers", runnable.Instance())
	options := map[string]int{}
	require.NoError(t, runnable.Options().Decode(&options))
	require.Equal(t, map[string]int{"poll": 10}, options)

	deps, err := runnable.Dependents()
	require.NoError(t, err)
	require.Equal(t, Runnables{}, deps)
}

func TestRunnableWithDepends(t *testing.T) {
	v := types.DecodeInterfaceSpec("group/0.1")
	Register("group", v, func(spec types.Spec) (Runnables, error) {
		// This just echos back whatever comes in
		return Runnables{AsRunnable(spec)}, nil
	})

	runnable := AsRunnable(mustSpec(types.SpecFromString(`
kind: group
version: group/0.1
metadata:
  name: workers
properties:
  max: 100
  min: 10
options:
  poll: 10
`)))

	require.Equal(t, "group", runnable.Kind())
	require.Equal(t, plugin.Name("group/workers"), runnable.Plugin())
	require.Equal(t, "workers", runnable.Instance())
	options := map[string]int{}
	require.NoError(t, runnable.Options().Decode(&options))
	require.Equal(t, map[string]int{"poll": 10}, options)

	deps, err := runnable.Dependents()
	require.NoError(t, err)
	require.Equal(t, Runnables{runnable}, deps)
}

func TestRunnablesFromSpec(t *testing.T) {

	Register("group", types.InterfaceSpec{Name: "Group"}, func(spec types.Spec) (Runnables, error) {
		if spec.Properties == nil {
			return nil, nil
		}

		type t struct {
			Instance struct {
				Plugin     plugin.Name
				Properties *types.Any
				Options    *types.Any
			}
			Flavor struct {
				Plugin     plugin.Name
				Properties *types.Any
				Options    *types.Any
			}
		}

		groupSpec := t{}
		err := spec.Properties.Decode(&groupSpec)
		if err != nil {
			return nil, err
		}

		return Runnables{
			AsRunnable(types.Spec{
				Kind: groupSpec.Instance.Plugin.Lookup(),
				Metadata: types.Metadata{
					Name: groupSpec.Instance.Plugin.String(),
				},
				Properties: groupSpec.Instance.Properties,
				Options:    groupSpec.Instance.Options,
			}),
			AsRunnable(types.Spec{
				Kind: groupSpec.Flavor.Plugin.Lookup(),
				Metadata: types.Metadata{
					Name: groupSpec.Flavor.Plugin.String(),
				},
				Properties: groupSpec.Flavor.Properties,
				Options:    groupSpec.Flavor.Options,
			}),
		}, nil
	})

	runnables, err := RunnablesFrom(mustSpecs(types.SpecsFromString(`
- kind: group
  version: Group
  metadata:
    name: us-east-compute/workers
  properties:
    Allocation:
      Size: 2
    Flavor:
      Plugin: vanilla
      Properties:
        Attachments:
          - ID: attachid
            Type: attachtype
        Init:
          - docker pull nginx:alpine
          - docker run -d -p 80:80 nginx-alpine
        Tags:
          project: infrakit
          tier: web
    Instance:
      Plugin: simulator/compute
      Properties:
        Note: Instance properties version 1.0
  options:
    poll: 10
`)))

	require.NoError(t, err)
	require.Equal(t, "group", runnables[0].Kind())
	require.Equal(t, "us-east-compute", runnables[0].Plugin().Lookup())
	require.Equal(t, "simulator", runnables[1].Kind())
	require.Equal(t, "simulator", runnables[1].Plugin().Lookup())
	require.Equal(t, "vanilla", runnables[2].Kind())
	require.Equal(t, "vanilla", runnables[2].Plugin().Lookup())

	runnables, err = RunnablesFrom(mustSpecs(types.SpecsFromString(`
- kind: group
  version: Group
  metadata:
    name: us-east-compute/workers
  properties:
    Allocation:
      Size: 2
    Flavor:
      Plugin: vanilla
      Properties:
        Attachments:
          - ID: attachid
            Type: attachtype
        Init:
          - docker pull nginx:alpine
          - docker run -d -p 80:80 nginx-alpine
        Tags:
          project: infrakit
          tier: web
    Instance:
      Plugin: simulator/compute
      Properties:
        Note: Instance properties version 1.0
  options:
    poll: 10

- kind: ingress
  metadata:
    name: us-east-net/workers.com
  properties:
    routes: 10
  options:
    poll: 20
`)))

	require.NoError(t, err)
	require.Equal(t, "group", runnables[0].Kind())
	require.Equal(t, "us-east-compute", runnables[0].Plugin().Lookup())
	require.Equal(t, "ingress", runnables[1].Kind())
	require.Equal(t, "us-east-net", runnables[1].Plugin().Lookup())
	require.Equal(t, "simulator", runnables[2].Kind())
	require.Equal(t, "simulator", runnables[2].Plugin().Lookup())
	require.Equal(t, "vanilla", runnables[3].Kind())
	require.Equal(t, "vanilla", runnables[3].Plugin().Lookup())

	require.Equal(t, "{\"poll\":10}", runnables[0].Options().String())
	options := map[string]interface{}{}
	require.NoError(t, runnables[1].Options().Decode(&options))
	require.Equal(t, float64(20), options["poll"])

	runnables, err = RunnablesFrom(mustSpecs(types.SpecsFromString(`
- kind: ingress
  metadata:
    name: us-east-net/workers.com
  properties:
    routes: 10
  options:
    poll: 20

# The lookup will be nfs (a plugin endpoint)
- kind: simulator/disk
  metadata:
    name: nfs/disk1
  properties:
    cidr: 10.20.100.100/16

# The lookup will default to simulator because it's not in the metadata/name
- kind: simulator/net
  metadata:
    name: subnet1
  properties:
    cidr: 10.20.100.100/16


`)))

	require.NoError(t, err)
	require.Equal(t, "ingress", runnables[0].Kind())
	require.Equal(t, "us-east-net", runnables[0].Plugin().Lookup())
	require.Equal(t, "simulator", runnables[1].Kind())
	require.Equal(t, "nfs", runnables[1].Plugin().Lookup())
	require.Equal(t, "simulator", runnables[2].Kind())
	require.Equal(t, "simulator", runnables[2].Plugin().Lookup())

}
