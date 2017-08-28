package manager

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func testNames(t *testing.T, kind string, pn plugin.Name, spec string) {
	s := types.Spec{}
	require.NoError(t, types.AnyYAMLMust([]byte(spec)).Decode(&s))
	q := specQuery{s}
	require.Equal(t, pn, q.Plugin())
	require.Equal(t, kind, q.Kind())

}

func TestDerivePluginNames(t *testing.T) {

	testNames(t, "instance-aws", plugin.Name("instance-aws/ec2-instance"), `
kind:      instance-aws/ec2-instance
version:   instance/v0.1.0
metadata:
  name: host1
  tags:
    role:    worker
    project: test
properties:
    instanceType: c2xlarge
    ami:          ami-12345
options:
    region: us-west-1
    stack:  test

`)

	testNames(t, "group", plugin.Name("group/workers"), `
kind:      group
metadata:
  name: workers
  tags:
    role:    worker
    project: test
properties:
    instanceType: c2xlarge
    ami:          ami-12345
options:
    region: us-west-1
    stack:  test

`)
}
