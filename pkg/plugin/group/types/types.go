package types

import (
	"crypto/sha1"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Spec is the configuration schema for the plugin, provided in group.Spec.Properties
type Spec struct {
	Instance   InstancePlugin
	Flavor     FlavorPlugin
	Allocation AllocationMethod
}

// AllocationMethod defines the type of allocation and supervision needed by a flavor's Group.
type AllocationMethod struct {
	Size       uint
	LogicalIDs []instance.LogicalID
}

// Index is the index of the instance's creation.  It provides a context for knowing
// what is being created.
type Index struct {

	// Group is the name of the group
	Group group.ID

	// Sequence is a sequence number that's per instance.
	Sequence uint
}

// InstancePlugin is the structure that describes an instance plugin.
type InstancePlugin struct {
	Plugin     plugin.Name
	Properties *types.Any // this will be the Spec of the plugin
}

// FlavorPlugin describes the flavor configuration
type FlavorPlugin struct {
	Plugin     plugin.Name
	Properties *types.Any // this will be the Spec of the plugin
}

// ParseProperties parses the group plugin properties JSON document in a group configuration.
func ParseProperties(config group.Spec) (Spec, error) {
	parsed := Spec{}
	if config.Properties != nil {
		if err := config.Properties.Decode(&parsed); err != nil {
			return parsed, fmt.Errorf("Invalid properties: %s", err)
		}
	}
	return parsed, nil
}

// UnparseProperties composes group.spec from id and props
func UnparseProperties(id string, props Spec) (group.Spec, error) {
	unparsed := group.Spec{ID: group.ID(id)}
	any, err := types.AnyValue(props)
	if err != nil {
		return unparsed, err
	}
	unparsed.Properties = any
	return unparsed, nil
}

// MustParse can be wrapped over ParseProperties to panic if parsing fails.
func MustParse(s Spec, e error) Spec {
	if e != nil {
		panic(e)
	}
	return s
}

func stableFormat(v interface{}) []byte {
	// Marshal the JSON to ensure stable key ordering.  This allows structurally-identical JSON to yield the same
	// hash even if the fields are reordered.

	unstable, err := json.Marshal(v)
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
	return stable
}

// InstanceHash computes a stable hash of the document in InstancePluginProperties.
func (c Spec) InstanceHash() string {
	// TODO(wfarner): This does not consider changes made by plugins that are not represented by user
	// configuration changes, such as if a plugin is updated.  We may be able to address this by resolving plugin
	// names to a versioned plugin identifier.

	hasher := sha1.New()
	hasher.Write(stableFormat(c.Instance))
	hasher.Write(stableFormat(c.Flavor))
	encoded := base32.StdEncoding.EncodeToString(hasher.Sum(nil))
	// Only valid characters are [a-z][0-9] for support on specific platforms
	encoded = strings.ToLower(encoded)
	// Remove extra padding
	encoded = strings.TrimRight(encoded, "=")
	return encoded
}
