package swarm

import (
	"testing"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseSpec(t *testing.T) {

	properties := ingress.Properties{
		{
			Vhost:    ingress.Vhost("test.com"),
			L4Plugin: plugin.Name("ingress/elb1"),
			RouteSources: map[string]*types.Any{
				"swarm": types.AnyValueMust(
					Spec{
						Docker: ConnectInfo{
							Host: "/var/run/docker.sock",
						},
					},
				),
			},
		},
	}

	yaml, err := types.AnyValueMust(properties).MarshalYAML()
	require.NoError(t, err)
	t.Log(string(yaml))

	properties2 := ingress.Properties{}

	err = types.AnyYAMLMust(yaml).Decode(&properties2)
	require.NoError(t, err)
	require.Equal(t, 1, len(properties2))
	spec := Spec{}
	err = properties[0].RouteSources["swarm"].Decode(&spec)
	require.NoError(t, err)
	require.Equal(t, "/var/run/docker.sock", spec.Docker.Host)
}
