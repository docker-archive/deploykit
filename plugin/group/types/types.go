package types

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/infrakit/spi/group"
)

// Spec is the configuration schema for the plugin, provided in group.Spec.Properties
type Spec struct {
	Instance InstancePlugin
	Flavor   FlavorPlugin
}

// InstancePlugin is the structure that describes an instance plugin.
type InstancePlugin struct {
	Plugin     string
	Properties *json.RawMessage // this will be the Spec of the plugin
}

// FlavorPlugin describes the flavor configuration
type FlavorPlugin struct {
	Plugin     string
	Properties *json.RawMessage // this will be the Spec of the plugin
}

// ParseProperties parses the group plugin properties JSON document in a group configuration.
func ParseProperties(config group.Spec) (Spec, error) {
	parsed := Spec{}
	if err := json.Unmarshal([]byte(RawMessage(config.Properties)), &parsed); err != nil {
		return parsed, fmt.Errorf("Invalid properties: %s", err)
	}
	return parsed, nil
}

// MustParse can be wrapped over ParseProperties to panic if parsing fails.
func MustParse(s Spec, e error) Spec {
	if e != nil {
		panic(e)
	}
	return s
}

// InstanceHash computes a stable hash of the document in InstancePluginProperties.
func (c Spec) InstanceHash() string {
	// Marshal the JSON to ensure stable key ordering.  This allows structurally-identical JSON to yield the same
	// hash even if the fields are reordered.

	// TODO(wfarner): This does not consider changes made by plugins that are not represented by user
	// configuration changes, such as if a plugin is updated.

	// TODO(wfarner): This does not consider flavor plugin and properties.  At present, since details like group
	// size and logical IDs are extracted from opaque properties, there's no way to distinguish between a group size
	// change and a change requiring a rolling update.

	unstable, err := json.Marshal(c.Instance)
	if err != nil {
		panic(err)
	}

	props := map[string]interface{}{}
	err = json.Unmarshal(unstable, &props)
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
