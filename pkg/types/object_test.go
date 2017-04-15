package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestObject(t *testing.T) {

	text := `
- class:        instance-aws/ec2-instance
  spiVersion:   instance/v0.1.0
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
    - class: instance-aws/ec2-volume
      name: disk1
      bind:
         volume/id : metadata/UID
         volume/size: properties/sizeGb

- class:        instance-aws/ec2-volume
  spiVersion:   instance/v0.1.0
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

	specs := []*Spec{}
	any, err := AnyYAML([]byte(text))
	require.NoError(t, err)
	err = any.Decode(&specs)
	require.NoError(t, err)
	require.Equal(t, 2, len(specs))

	objects, err := Instantiate(specs,
		func(spec *Spec) error {
			return nil
		}, func(spec *Spec) []interface{} {
			return []interface{}{spec.Class, spec.Metadata.Name}
		})

	require.NoError(t, err)

	T(100).Infoln("objects=", objects)
	disk := objects.FindBy("instance-aws/ec2-volume", "disk1")
	require.NotNil(t, disk)

	disk.Metadata.Identity = &Identity{UID: "disk-11234"}

	host := objects.FindBy("instance-aws/ec2-volume", "host1")
	require.Nil(t, host) // wrong class

	host = objects.FindBy("instance-aws/ec2-instance", "host1")
	require.NotNil(t, host)

	other, m, err := host.ResolveDepends(objects,
		func(o *Object) []interface{} {
			return []interface{}{o.Class, o.Metadata.Name}
		})

	require.NoError(t, err)

	T(100).Infoln(m)

	require.Equal(t, "disk-11234", m["volume/id"])
	require.Equal(t, float64(100), m["volume/size"])
	require.True(t, len(other) > 0)
}
