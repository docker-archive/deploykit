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

// GroupKind is the identifier for the type of supervision needed by a group.
type GroupKind int

// Definitions of supported role types.
const (
	KindUnknown   GroupKind = iota
	KindDynamicIP GroupKind = iota
	KindStaticIP  GroupKind = iota
)

// ProvisionDetails are the parameters that will be used to provision a machine.
type ProvisionDetails struct {
	BootScript string
	Tags       map[string]string
	PrivateIP  *string
	Volume     *instance.VolumeID
}

// A ProvisionHelper defines custom behavior for provisioning instances.
type ProvisionHelper interface {

	// Validate checks whether the helper can support a configuration.
	Validate(config group.Configuration, parsed Schema) error

	// GroupKind translates the helper's role names into Roles that define how the group is managed.  This allows
	// a helper to define specialized roles and customize those machines accordingly in PreProvision().
	GroupKind(roleName string) GroupKind

	// PreProvision allows the helper to modify the provisioning instructions for an instance.  For example, a
	// helper could be used to place additional tags on the machine, or generate a specialized BootScript based on
	// the machine configuration.
	PreProvision(config group.Configuration, details ProvisionDetails) (ProvisionDetails, error)

	// Healthy determines whether an instance is healthy.
	Healthy(inst instance.Description) (bool, error)
}

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
	IPs                      []string
	InstancePlugin           string
	InstancePluginProperties json.RawMessage
}

// InstanceHash computes a stable hash of the document in InstancePluginProperties.
func (c Schema) InstanceHash() string {
	// TODO(wfarner): This does not take ProvisionHelper augmentations (e.g. tags, bootScript) into consideration.
	return instanceHash(c.InstancePluginProperties)
}

func instanceHash(config json.RawMessage) string {
	// First unmarshal and marshal the JSON to ensure stable key ordering.  This allows structurally-identical
	// JSON to yield the same hash even if the fields are reordered.

	props := map[string]interface{}{}
	err := json.Unmarshal(config, &props)
	if err != nil {
		panic(err)
	}

	stable, err := json.Marshal(props)
	if err != nil {
		panic(err)
	}

	hasher := sha1.New()
	hasher.Write(stable)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}
