package types

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeSpec(t *testing.T) {

	spec := `
kind:        instance-aws/ec2-instance
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

`
	s := Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	expected := Spec{
		Kind:    "instance-aws/ec2-instance",
		Version: "instance/v0.1.0",
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

	s.Version = ""
	require.Error(t, s.Validate())
}

func TestEncodeDecodeObject(t *testing.T) {

	object := `
kind:        instance-aws/ec2-instance
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
    - kind: instance-aws/ebs-volume
      name: /var/lib/docker
      bind:
         volume/id : Spec/Metadata/Identity/ID

state:
    instanceType: c2xlarge
    ami:          ami-12345
    instanceState: running
`
	o := Object{}
	require.NoError(t, yaml.Unmarshal([]byte(object), &o))

	urlStr := "https://playbooks.test.com/aws-instance-template.ikt"
	templateURL, _ := NewURL(urlStr)

	expected := Object{
		Spec: Spec{
			Kind:    "instance-aws/ec2-instance",
			Version: "instance/v0.1.0",
			Metadata: Metadata{
				Identity: &Identity{
					ID: "u-12134",
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
						"volume/id": PointerFromString("Spec/Metadata/Identity/ID"),
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
	require.Equal(t, "u-12134", o.Spec.Metadata.Identity.ID)
	require.Equal(t, "instance/v0.1.0", o.Spec.Version)
	require.True(t, o.Template.Absolute())
	require.Equal(t, urlStr, o.Template.Value().String())

	require.NoError(t, o.Validate())

	o.Metadata.Identity.ID = ""
	require.Error(t, o.Validate())
}

func TestMetadata(t *testing.T) {
	require.Equal(t, (Metadata{}).Fingerprint(), (Metadata{}).Fingerprint())
	require.Equal(t, (Metadata{Name: "foo"}).Fingerprint(), (Metadata{Name: "foo"}).Fingerprint())
	require.NotEqual(t, (Metadata{Name: "foo"}).Fingerprint(), (Metadata{}).Fingerprint())
}

func TestComparable(t *testing.T) {

	require.Equal(t, 0, (Identity{ID: "1"}).Compare(Identity{ID: "1"}))
	require.Equal(t, -1, (Identity{ID: "1"}).Compare(Identity{ID: "2"}))
	require.Equal(t, 1, (Identity{ID: "2"}).Compare(Identity{ID: "1"}))

	require.Equal(t, 0, (Metadata{Name: "1"}).Compare(Metadata{Name: "1"}))
	require.Equal(t, 1, (Metadata{Name: "2"}).Compare(Metadata{Name: "1"}))
	require.Equal(t, -1, (Metadata{Name: "1"}).Compare(Metadata{Name: "2"}))
	require.Equal(t, -1, (Metadata{Identity: &Identity{ID: "1"}, Name: "1"}).Compare(
		Metadata{Identity: &Identity{ID: "2"}, Name: "1"}))

	// This case the name isn't as important as the identity.  This applies to the case where
	// the name is a typed plugin name (eg. simulator/disk) but the identity is "mydisk1".
	require.Equal(t, -1, (Metadata{Identity: &Identity{ID: "1"}, Name: "1"}).Compare(
		Metadata{Identity: &Identity{ID: "2"}, Name: "1"}))

	require.Equal(t, 0, (Metadata{Name: "1", Tags: map[string]string{"a": "b"}}).Compare(
		Metadata{Name: "1", Tags: map[string]string{"a": "b"}}))
	require.Equal(t, -1, (Metadata{Name: "1", Tags: map[string]string{"a": "a"}}).Compare(
		Metadata{Name: "1", Tags: map[string]string{"a": "b"}}))
	require.Equal(t, 1, (Metadata{Name: "1", Tags: map[string]string{"a": "c"}}).Compare(
		Metadata{Name: "1", Tags: map[string]string{"a": "b"}}))
	require.Equal(t, 1, (Metadata{Name: "1", Tags: map[string]string{"x": "c"}}).Compare(
		Metadata{Name: "1", Tags: map[string]string{"a": "b"}}))

	require.Equal(t, 0, (Spec{
		Kind:    "group",
		Version: "Group/0.1.0",
		Metadata: Metadata{
			Name: "group/workers",
		},
		Properties: AnyValueMust(map[string]interface{}{
			"count": 100,
			"type":  "foo",
		}),
		Options: AnyValueMust(map[string]interface{}{
			"poll": true,
		}),
	}).Compare(Spec{
		Kind:    "group",
		Version: "Group/0.1.0",
		Metadata: Metadata{
			Name: "group/workers",
		},
		Properties: AnyValueMust(map[string]interface{}{
			"count": 100,
			"type":  "foo",
		}),
		Options: AnyValueMust(map[string]interface{}{
			"poll": true,
		}),
	}))

	require.Equal(t, -1, (Spec{
		Kind:    "simulator/subnet",
		Version: "Instance/0.1.0",
		Metadata: Metadata{
			Identity: &Identity{ID: "subnet1"},
			Name:     "us-east",
		},
		Properties: AnyValueMust(map[string]interface{}{
			"cidr": "10.20.100.100/16",
		}),
		Options: AnyValueMust(map[string]interface{}{
			"flag": true,
		}),
	}).Compare(Spec{
		Kind:    "simulator/subnet",
		Version: "Instance/0.1.0",
		Metadata: Metadata{
			Identity: &Identity{ID: "subnet2"},
			Name:     "us-east",
		},
		Properties: AnyValueMust(map[string]interface{}{
			"cidr": "10.20.100.100/16",
		}),
		Options: AnyValueMust(map[string]interface{}{
			"flag": true,
		}),
	}))
}
