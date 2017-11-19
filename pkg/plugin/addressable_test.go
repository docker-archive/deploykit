package plugin

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func testNames(t *testing.T, kind string, pn Name, instance string, spec string) {
	s := types.Spec{}
	require.NoError(t, types.AnyYAMLMust([]byte(spec)).Decode(&s))
	q := AsAddressable(s)
	require.Equal(t, pn, q.Plugin())
	require.Equal(t, kind, q.Kind())
	require.Equal(t, instance, q.Instance())
}

func TestAddressable(t *testing.T) {
	a := NewAddressableFromMetadata("group", types.Metadata{Name: "workers"})
	require.Equal(t, "group", a.Kind())
	require.Equal(t, "group/workers", string(a.Plugin()))
	require.Equal(t, "workers", a.Instance())

	spec, err := types.SpecFromString(`
kind: group
metadata:
  name: workers
`)
	require.NoError(t, err)
	b := AsAddressable(spec)
	require.Equal(t, "group", b.Kind())
	require.Equal(t, "group/workers", string(b.Plugin()))
	require.Equal(t, "workers", b.Instance())

	c := NewAddressable("group", Name("group-stateless"), "")
	require.Equal(t, "group", c.Kind())
	require.Equal(t, "group/group-stateless", string(c.Plugin()))
	require.Equal(t, "group-stateless", c.Instance())

	c = NewAddressable("group", Name("group-stateless"), "mygroup")
	require.Equal(t, "group", c.Kind())
	require.Equal(t, "group/group-stateless", string(c.Plugin()))
	require.Equal(t, "mygroup", c.Instance())

	c = NewAddressable("group", Name("group-stateless/"), "")
	require.Equal(t, "group", c.Kind())
	require.Equal(t, "group-stateless", string(c.Plugin()))
	require.Equal(t, "", c.Instance())

	c = NewAddressable("group", Name("group-stateless/"), "mygroup")
	require.Equal(t, "group", c.Kind())
	require.Equal(t, "group-stateless/mygroup", string(c.Plugin()))
	require.Equal(t, "mygroup", c.Instance())

	c = NewAddressableFromPluginName(Name("swarm/manager"))
	require.Equal(t, "swarm", c.Kind())
	require.Equal(t, "swarm/manager", string(c.Plugin()))
	require.Equal(t, "swarm", c.Plugin().Lookup())
}

func TestDerivePluginNames(t *testing.T) {
	testNames(t, "ingress", Name("ingress/lb1"), "lb1", `
kind: ingress
metadata:
  name: lb1
`)
	testNames(t, "ingress", Name("us-east/lb1"), "lb1", `
kind: ingress
metadata:
  name: us-east/lb1
`)
	testNames(t, "group", Name("group/workers"), "workers", `
kind: group
metadata:
  name: workers
`)
	testNames(t, "group", Name("group/workers"), "workers", `
kind: group
metadata:
  name: group/workers
`)
	testNames(t, "group", Name("us-east/workers"), "workers", `
kind: group
metadata:
  name: us-east/workers
`)
	testNames(t, "resource", Name("resource/vpc1"), "vpc1", `
kind: resource
metadata:
  name: vpc1
`)
	testNames(t, "resource", Name("us-east/vpc1"), "vpc1", `
kind: resource
metadata:
  name: us-east/vpc1
`)
	testNames(t, "simulator", Name("simulator/disk"), "disk1", `
kind: simulator/disk
metadata:
  name: disk1
`)
	testNames(t, "simulator", Name("us-east/disk"), "disk1", `
kind: simulator/disk
metadata:
  name: us-east/disk1
`)
	testNames(t, "aws", Name("aws/ec2-instance"), "host1", `
kind: aws/ec2-instance
metadata:
  name: host1
`)
	testNames(t, "aws", Name("us-east/ec2-instance"), "host1", `
kind: aws/ec2-instance
metadata:
  name: us-east/host1
`)
}
