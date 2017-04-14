package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestParseProperties(t *testing.T) {
	properties := types.AnyString(`{
  "NamePrefix": "foo",
  "Region": "nyc3",
  "Size": "512mb",
  "Image": "ubuntu-14-04-x64",
  "Backups": false,
  "Ipv6": true,
  "Private_networking": null,
  "Tags": ["foo"]
}`)

	p, err := ParseProperties(properties)
	assert.NoError(t, err)
	assert.Equal(t, p, Properties{
		NamePrefix:        "foo",
		Size:              "512mb",
		Image:             "ubuntu-14-04-x64",
		Backups:           false,
		IPv6:              true,
		PrivateNetworking: false,
		Tags:              []string{"foo"},
	})
}

func TestParsePropertiesFail(t *testing.T) {
	properties := types.AnyString(`{
  "NamePrefix": "bar",
  "tags": {
    "foo": "bar",
  }
}`)

	_, err := ParseProperties(properties)
	assert.Error(t, err)
}

func TestParseTags(t *testing.T) {
	id := instance.LogicalID("foo")
	spec := instance.Spec{
		Tags: map[string]string{
			"foo":    "bar",
			"banana": "",
		},
		LogicalID: &id,
	}

	tags, err := ParseTags(spec)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo":             "bar",
		"banana":          "",
		InfrakitLogicalID: string(id),
		InfrakitDOVersion: InfrakitDOCurrentVersion,
	}, tags)
}
