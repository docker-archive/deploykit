package types

import (
	"testing"

	"github.com/stretchr/testify/require"
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
         volume/id : metadata/ID
         volume/size: properties/sizeGb

- kind:        instance-aws/ec2-volume
  version:   instance/v0.1.0
  metadata:
    id: disk1-1234
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
  state:
    sizeGb : 100
    type:    ssd
    status: online
`

	objects := []*Object{}
	any, err := AnyYAML([]byte(text))
	require.NoError(t, err)
	err = any.Decode(&objects)
	require.NoError(t, err)
	require.Equal(t, 2, len(objects))

	require.NoError(t, objects[1].Validate())
	require.Error(t, objects[0].Validate())
	require.Nil(t, objects[0].State)
	require.NotNil(t, objects[1].State)
}
