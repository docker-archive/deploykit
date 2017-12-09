package core

import (
	"fmt"
	"sort"
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestObject(t *testing.T) {

	text := `
- kind:        instance-aws/ec2-instance
  version:   instance/v0.1.0
  metadata:
    name: host1
    tags:
      role:    worker
      project: test
  template: https://playbooks.test.com/aws-instance-template.ikt
  properties:
    instanceType: c2xlarge
    ami:          ami-12345
    volume:  "{{ var \"volume/id\" }}"
  options:
    region: us-west-1
    stack:  test
  depends:
    - kind: instance-aws/ec2-volume
      name: disk1
      bind:
         volume/id : metadata/id
         volume/size: properties/sizeGb

- kind:        instance-aws/ec2-volume
  version:   instance/v0.1.0
  metadata:
    name: disk1
    tags:
      role:    worker
      project: test
  template: https://playbooks.test.com/aws-volume-template.ikt
  properties:
    sizeGb : 100
    type:    ssd
  options:
    region: us-west-1
    stack:  test
`

	specs := []*types.Spec{}
	any, err := types.AnyYAML([]byte(text))
	require.NoError(t, err)
	err = any.Decode(&specs)
	require.NoError(t, err)
	require.Equal(t, 2, len(specs))

	objects := NewObjects(func(o *types.Object) []interface{} {
		return []interface{}{o.Spec.Kind, o.Spec.Metadata.Name}
	})

	objects.Add(&types.Object{
		Spec: *specs[0],
	})
	objects.Add(&types.Object{
		Spec: *specs[1],
	})

	T(100).Infoln("objects=", objects)
	disk := objects.FindBy("instance-aws/ec2-volume", "disk1")
	require.NotNil(t, disk)

	disk.Metadata.Identity = &types.Identity{ID: "disk-11234"}

	host := objects.FindBy("instance-aws/ec2-volume", "host1")
	require.Nil(t, host) // wrong kind

	host = objects.FindBy("instance-aws/ec2-instance", "host1")
	require.NotNil(t, host)

	m, err := resolveDepends(host, objects)
	require.NoError(t, err)

	T(100).Infoln(m)

	require.Equal(t, "disk-11234", m["volume/id"])
	require.Equal(t, float64(100), m["volume/size"])

	m, err = resolveDepends(disk, objects)
	require.NoError(t, err)

	T(100).Infoln(m)
}

func TestObjectNested(t *testing.T) {

	text := `
- kind:        group
  version:   group/v0.1.0
  metadata:
    name: managers
    tags:
      role:    managers
      project: test
  properties:
    instance:
      kind:        instance-aws/ec2-instance
      version:   instance/v0.1.0
      metadata:
        name: manager-node
        tags:
          role:    manager
          project: test
      template: aws-instance-template.yml
      properties:
        instanceType: c2xlarge
        ami:          ami-12345
        volume:  "{{ var \"volumeId\" }}"
      options:
        region: us-west-1
        stack:  test
      depends:
        - kind: instance-aws/ec2-volume
          name: disk1
          bind:
            disk1VolumeId : metadata/id
        - kind: instance-aws/ec2-volume
          name: disk2
          bind:
            disk2VolumeId : metadata/id
        - kind: instance-aws/ec2-volume
          name: disk3
          bind:
            disk3VolumeId : metadata/id

    flavor:
      kind:        flavor-swarm/manager
      version:   flavor/v0.1.0
      metadata:
        name: swarm-manager
        tags:
          role:    manager
          project: test
      template: swarm-manager.yml
      options:
          region: us-west-1
          stack:  test
- kind:        instance-aws/ec2-volume
  version:   instance/v0.1.0
  metadata:
    name: disk1
    tags:
      role:    manager
      project: test
  template: aws-volume-template.yml
  properties:
    sizeGb : 100
    type:    ssd
  options:
    region: us-west-1
    stack:  test
- kind:        instance-aws/ec2-volume
  version:   instance/v0.1.0
  metadata:
    name: disk2
    tags:
      role:    manager
      project: test
  template: https://playbooks.test.com/aws-volume-template.ikt
  properties:
    sizeGb : 100
    type:    ssd
  options:
    region: us-west-1
    stack:  test
- kind:        instance-aws/ec2-volume
  version:   instance/v0.1.0
  metadata:
    name: disk3
    tags:
      role:    manager
      project: test
  template: https://playbooks.test.com/aws-volume-template.ikt
  properties:
    sizeGb : 100
    type:    ssd
  options:
    region: us-west-1
    stack:  test
`

	specs := []*types.Spec{}
	any, err := types.AnyYAML([]byte(text))
	require.NoError(t, err)
	err = any.Decode(&specs)
	require.NoError(t, err)
	require.Equal(t, 4, len(specs)) // nested

	objects := NewObjects(func(o *types.Object) []interface{} {
		return []interface{}{o.Spec.Kind, o.Spec.Metadata.Name}
	})

	all := types.AllSpecs(specs) // all including nested
	require.Equal(t, 6, len(all))

	// dependency order
	ordered, err := types.OrderByDependency(all)
	require.NoError(t, err)

	found := []string{}
	for i := 0; i < 3; i++ {
		found = append(found, ordered[i].Metadata.Name)
	}

	sort.Strings(found)
	require.Equal(t, []string{"disk1", "disk2", "disk3"}, found)

	// instantiate the object
	for _, spec := range all {
		objects.Add(&types.Object{Spec: *spec})
	}

	require.Equal(t, 6, objects.Len())

	T(100).Infoln("objects=", objects)

	for i, n := range []string{"disk1", "disk2", "disk3"} {
		disk := objects.FindBy("instance-aws/ec2-volume", n)
		require.NotNil(t, disk)
		disk.Metadata.Identity = &types.Identity{ID: fmt.Sprintf("disk-%d", i)}
	}

	node := objects.FindBy("instance-aws/ec2-instance", "manager-node")
	require.NotNil(t, node)

	m, err := resolveDepends(node, objects)
	require.NoError(t, err)

	T(100).Infoln(m)
}
