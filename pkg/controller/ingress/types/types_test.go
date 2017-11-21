package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestSpecParsing(t *testing.T) {

	any, err := types.AnyYAML([]byte(`
kind: ingress
metadata:
  name: test.com
  tags:
    project: testing
    user: chungers

options:
  SyncInterval: 10s  # syntax is a string form of Go time.Duration

properties:
  - Backends:
      Groups:
        - group/workers # This is a group at socket(group), groupID(workers).

    L4Plugin: simulator/lb1
    Routes:
      - LoadBalancerPort: 80
        LoadBalancerProtocol: https
        Port: 8080
        Protocol: http
`))

	require.NoError(t, err)

	spec := types.Spec{}

	err = any.Decode(&spec)
	require.NoError(t, err)

	specs := []Spec{}
	err = spec.Properties.Decode(&specs)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTPS, specs[0].Routes[0].LoadBalancerProtocol)

	require.Error(t, specs[0].Validate())
}
