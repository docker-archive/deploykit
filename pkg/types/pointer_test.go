package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPointer(t *testing.T) {

	pointer1 := PointerFromString("a/b/c/d")
	pointer2 := PointerFromPath(PathFrom("a", "b", "c", "d"))
	require.Equal(t, pointer1, pointer2)

	type test struct {
		Pointer    Pointer  `json:"pointer"`
		PointerPtr *Pointer `json:"pointerPtr"`
	}

	input := `
{
	"pointer" : "foo/bar/baz",
	"pointerPtr" : "github.com/docker/infrakit/pkg/testing"
}
`

	decoded := test{}

	err := AnyString(input).Decode(&decoded)
	require.NoError(t, err)

	require.Equal(t, PointerFromString("foo/bar/baz").String(), decoded.Pointer.String())
	require.Equal(t, PointerFromString("github.com/docker/infrakit/pkg/testing").String(), decoded.PointerPtr.String())

	any := AnyValueMust(decoded)
	require.Equal(t, `{
"pointer": "foo/bar/baz",
"pointerPtr": "github.com/docker/infrakit/pkg/testing"
}`, any.String())

	text := `
class:        instance-aws/ec2-instance
version:   instance/v0.1.0
metadata:
  id : u-12134
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
    - class: instance-aws/ebs-volume
      name: /var/lib/docker
      bind:
         volume/id : Spec/Metadata/Identity/ID

state:
    instanceType: c2xlarge
    ami:          ami-12345
    instanceState: running
`

	any = AnyBytes(nil)
	err = any.UnmarshalYAML([]byte(text))
	require.NoError(t, err)

	buff, err := any.MarshalYAML()
	require.NoError(t, err)

	object1, object2 := Object{}, Object{}

	any1, err := AnyYAML([]byte(text))
	require.NoError(t, err)

	any2, err := AnyYAML(buff)
	require.NoError(t, err)

	err = any1.Decode(&object1)
	require.NoError(t, err)

	err = any2.Decode(&object2)
	require.NoError(t, err)

	require.Equal(t, object1, object2)

	require.Equal(t, "u-12134", object1.Metadata.Identity.ID)
}
