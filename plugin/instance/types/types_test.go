package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
)

func TestParseProperties(t *testing.T) {
	properties := types.AnyString(`{
  "name": "example.com",
  "region": "nyc3",
  "size": "512mb",
  "image": "ubuntu-14-04-x64",
  "ssh_keys": null,
  "backups": false,
  "ipv6": true,
  "user_data": null,
  "private_networking": null,
  "volumes": null,
  "tags": ["foo"]
}`)

	p, err := ParseProperties(properties)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(p)
}
