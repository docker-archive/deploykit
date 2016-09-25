package types

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
)

// These types are declared in a separate package to break an import cycle between group (test) -> mock -> group.

// ParseProperties parses the group plugin properties JSON document in a group configuration.
func ParseProperties(config group.Configuration) (Schema, error) {
	parsed := Schema{}
	if err := json.Unmarshal([]byte(config.Properties), &parsed); err != nil {
		return parsed, fmt.Errorf("Invalid properties: %s", err)
	}
	return parsed, nil
}

// MustParse can be wrapped over ParseProperties to panic if parsing fails.
func MustParse(s Schema, e error) Schema {
	if e != nil {
		panic(e)
	}
	return s
}

// Schema is the document schema for the plugin, provided in group.Configuration.
type Schema struct {
	Size                     uint32
	LogicalIDs               []instance.LogicalID
	InstancePlugin           string
	InstancePluginProperties *json.RawMessage
	FlavorPlugin             string
	FlavorPluginProperties   *json.RawMessage
}

// InstanceHash computes a stable hash of the document in InstancePluginProperties.
func (c Schema) InstanceHash() string {
	// TODO(wfarner): This does not take ProvisionHelper augmentations (e.g. tags, bootScript) into consideration.
	return instanceHash(RawMessage(c.InstancePluginProperties))
}

// RawMessage converts a pointer to a raw message to a copy of the value. If the pointer is nil, it returns
// an empty raw message.  This is useful for structs where fields are json.RawMessage pointers for bi-directional
// marshal and unmarshal (value receivers will encode base64 instead of raw json when marshaled), so bi-directional
// structs should use pointer fields.
func RawMessage(r *json.RawMessage) (raw json.RawMessage) {
	if r != nil {
		raw = *r
	}
	return
}

func instanceHash(config json.RawMessage) string {
	// First unmarshal and marshal the JSON to ensure stable key ordering.  This allows structurally-identical
	// JSON to yield the same hash even if the fields are reordered.

	props := map[string]interface{}{}
	err := json.Unmarshal(config, &props)
	if err != nil {
		panic(err)
	}

	stable, err := json.MarshalIndent(props, "  ", "  ") // sorts the fields
	if err != nil {
		panic(err)
	}

	hasher := sha1.New()
	hasher.Write(stable)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}
