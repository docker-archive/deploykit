package depends

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestDepends(t *testing.T) {

	v := types.DecodeInterfaceSpec("Test/0.1")
	Register("test", v, func(spec types.Spec) (Runnables, error) {
		return Runnables{
			AsRunnable(mustSpec(types.SpecFromString(`
kind: simulator/compute
version: Instance/0.1
metadata:
  name: us-east1
options:
  poll: 10
`))),
			AsRunnable(mustSpec(types.SpecFromString(`
kind: swarm/manager
version: Flavor/0.1
metadata:
  name: swarm
options:
  docker: /var/run/docker.sock
`))),
		}, nil
	})

	found, err := Resolve(mustSpec(types.SpecFromString(``)), "test", &v)
	require.NoError(t, err)
	// in this case, the resolver always returns 2
	require.Equal(t, Runnables{
		AsRunnable(mustSpec(types.SpecFromString(`
kind: simulator/compute
version: Instance/0.1
metadata:
  name: us-east1
options:
  poll: 10
`))),
		AsRunnable(mustSpec(types.SpecFromString(`
kind: swarm/manager
version: Flavor/0.1
metadata:
  name: swarm
options:
  docker: /var/run/docker.sock
`))),
	}, found)

	found, err = Resolve(mustSpec(types.SpecFromString(``)), "nope", &v)
	require.NoError(t, err)
	require.Equal(t, 0, len(found))
}
