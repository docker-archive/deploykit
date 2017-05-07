package types

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeSpec(t *testing.T) {

	spec := `
kind:        instance-aws/ec2-instance
spiVersion:   instance/v0.1.0
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

`
	s := Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	expected := Spec{
		Kind:       "instance-aws/ec2-instance",
		SpiVersion: "instance/v0.1.0",
		Metadata: Metadata{
			Name: "host1",
			Tags: map[string]string{
				"role":    "worker",
				"project": "test",
			},
		},
		Properties: AnyValueMust(map[string]interface{}{
			"instanceType": "c2xlarge",
			"ami":          "ami-12345",
		}),
		Options: AnyValueMust(map[string]interface{}{
			"region": "us-west-1",
			"stack":  "test",
		}),
	}

	require.Equal(t, AnyValueMust(expected), AnyValueMust(s))
	require.NoError(t, s.Validate())

	s.Kind = ""
	require.Error(t, s.Validate())

	s.SpiVersion = ""
	require.Error(t, s.Validate())
}

func TestEncodeDecodeObject(t *testing.T) {

	object := `
kind:        instance-aws/ec2-instance
spiVersion:   instance/v0.1.0
metadata:
  uid : u-12134
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
    - kind: instance-aws/ebs-volume
      name: /var/lib/docker
      bind:
         volume/id : Spec/Metadata/Identity/UID

state:
    instanceType: c2xlarge
    ami:          ami-12345
    instanceState: running
`
	o := Object{}
	require.NoError(t, yaml.Unmarshal([]byte(object), &o))

	templateURL, _ := NewURL("https://playbooks.test.com/aws-instance-template.ikt")

	expected := Object{
		Spec: Spec{
			Kind:       "instance-aws/ec2-instance",
			SpiVersion: "instance/v0.1.0",
			Metadata: Metadata{
				Identity: &Identity{
					UID: "u-12134",
				},
				Name: "host1",
				Tags: map[string]string{
					"role":    "worker",
					"project": "test",
				},
			},
			Template: templateURL,
			Properties: AnyValueMust(map[string]interface{}{
				"instanceType": "c2xlarge",
				"ami":          "ami-12345",
				"volume":       "{{ var \"volume/id\" }}",
			}),
			Options: AnyValueMust(map[string]interface{}{
				"region": "us-west-1",
				"stack":  "test",
			}),
			Depends: []Dependency{
				{
					Kind: "instance-aws/ebs-volume",
					Name: "/var/lib/docker",
					Bind: map[string]*Pointer{
						"volume/id": PointerFromString("Spec/Metadata/Identity/UID"),
					},
				},
			},
		},
		State: AnyValueMust(map[string]interface{}{
			"instanceType":  "c2xlarge",
			"ami":           "ami-12345",
			"instanceState": "running",
		}),
	}

	require.Equal(t, AnyValueMust(expected), AnyValueMust(o))
	require.Equal(t, "u-12134", o.Spec.Metadata.Identity.UID)
	require.Equal(t, "instance/v0.1.0", o.Spec.SpiVersion)

	require.NoError(t, o.Validate())

	o.Metadata.Identity.UID = ""
	require.Error(t, o.Validate())
}
