package types

import (
	"crypto/sha1"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

func init() {
	depends.Register("group", types.InterfaceSpec(group.InterfaceSpec), ResolveDependencies)
}

// Options capture the options for starting up the group controller.
type Options struct {

	// Self is set when the controller is part of a group that can be updated.
	Self *instance.LogicalID

	// PollInterval is the frequency for syncing the state
	PollInterval types.Duration

	// MaxParallelNum is the max number of parallel instance operation. Default =0 (no limit)
	MaxParallelNum uint

	// PollIntervalGroupSpec polls for group spec at this interval to update the metadata paths
	PollIntervalGroupSpec types.Duration

	// PollIntervalGroupDetail polls for group details at this interval to update the metadata paths
	PollIntervalGroupDetail types.Duration
}

// ResolveDependencies returns a list of dependencies by parsing the opaque Properties blob.
func ResolveDependencies(spec types.Spec) (depends.Runnables, error) {

	if spec.Properties == nil {
		return nil, nil
	}

	// This extends on the group plugin spec to add optional Options section
	type t struct {
		Instance struct {
			Plugin     plugin.Name
			Properties *types.Any
			Options    *types.Any
		}
		Flavor struct {
			Plugin     plugin.Name
			Properties *types.Any
			Options    *types.Any
		}
	}

	groupSpec := t{}
	err := spec.Properties.Decode(&groupSpec)
	if err != nil {
		return nil, err
	}

	instancePlugin := types.Spec{
		Kind: groupSpec.Instance.Plugin.Lookup(),
		Metadata: types.Metadata{
			Name: groupSpec.Instance.Plugin.String(),
		},
		Properties: groupSpec.Instance.Properties,
		Options:    groupSpec.Instance.Options,
	}

	flavorPlugin := types.Spec{
		Kind: groupSpec.Flavor.Plugin.Lookup(),
		Metadata: types.Metadata{
			Name: groupSpec.Flavor.Plugin.String(),
		},
		Properties: groupSpec.Flavor.Properties,
		Options:    groupSpec.Flavor.Options,
	}

	all := depends.Runnables{
		depends.AsRunnable(instancePlugin),
		depends.AsRunnable(flavorPlugin),
	}

	// For any instance / flavor plugins nested:

	nestedInstances, err := depends.Resolve(instancePlugin, instancePlugin.Kind, nil)
	if err == nil {
		all = append(all, nestedInstances...)
	}
	nestedFlavors, err := depends.Resolve(flavorPlugin, flavorPlugin.Kind, nil)
	if err == nil {
		all = append(all, nestedFlavors...)
	}
	return all, nil
}

// Spec is the configuration schema for the plugin, provided in group.Spec.Properties
type Spec struct {
	Instance   InstancePlugin
	Flavor     FlavorPlugin
	Allocation group.AllocationMethod
	Updating   Updating
}

// Updating is the configuration schema using on a rolling update and defines how long
// a node must be healthy before the next node is updated. If Duration is set then the
// node must be healthy for at least the specified time. If Count is set then the
// node must be healthy for specified number of poll intervals. Both Duration and Count
// cannot be non 0.
type Updating struct {
	Duration                  types.Duration
	Count                     int
	SkipBeforeInstanceDestroy *SkipBeforeInstanceDestroy
}

// // AllocationMethod defines the type of allocation and supervision needed by a flavor's Group.
// type AllocationMethod struct {
// 	Size       uint
// 	LogicalIDs []instance.LogicalID
// }

// // Index is the index of the instance's creation.  It provides a context for knowing
// // what is being created.
// type Index struct {

// 	// Group is the name of the group
// 	Group group.ID

// 	// Sequence is a sequence number that's per instance.
// 	Sequence uint
// }

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

// SkipBeforeInstanceDestroy defines the action that should be skipped prior
// destroying the instance.
type SkipBeforeInstanceDestroy string

var (
	// SkipBeforeInstanceDestroyDrain is the policy that the node is not be
	// drained before the instance is destroyed
	SkipBeforeInstanceDestroyDrain = SkipBeforeInstanceDestroy("drain")
)

// MarshalJSON returns the json representation
func (s SkipBeforeInstanceDestroy) MarshalJSON() ([]byte, error) {
	v := string(s)
	if _, has := map[string]int{"drain": 1}[v]; !has {
		return nil, fmt.Errorf("invalid value %v", s)
	}
	return []byte("\"" + v + "\""), nil
}

// UnmarshalJSON unmarshals the buffer to this struct
func (s *SkipBeforeInstanceDestroy) UnmarshalJSON(buff []byte) error {
	parsed := strings.Trim(string(buff), "\"")
	switch parsed {
	case "drain":
		*s = SkipBeforeInstanceDestroyDrain
		return nil
	}
	return nil
}
