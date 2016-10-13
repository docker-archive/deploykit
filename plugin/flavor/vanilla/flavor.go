package vanilla

import (
	"encoding/json"
	"strings"

	"github.com/docker/infrakit/spi/flavor"
	"github.com/docker/infrakit/spi/instance"
)

// Spec is the model of the Properties section of the top level group spec.
type Spec struct {
	flavor.AllocationMethod

	// UserData
	UserData []string

	// Labels
	Labels map[string]string
}

// NewPlugin creates a Flavor plugin that doesn't do very much. It assumes instances are
// identical (cattles) but can assume specific identities (via the LogicalIDs).  The
// instances here are treated identically because we have constant UserData that applies
// to all instances
func NewPlugin() flavor.Plugin {
	return vanillaFlavor(0)
}

type vanillaFlavor int

func (f vanillaFlavor) Validate(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {
	s := Spec{}
	err := json.Unmarshal(flavorProperties, &s)
	return s.AllocationMethod, err
}

func (f vanillaFlavor) Healthy(inst instance.Description) (bool, error) {
	return true, nil
}

func (f vanillaFlavor) Prepare(flavor json.RawMessage, instance instance.Spec) (instance.Spec, error) {
	s := Spec{}
	err := json.Unmarshal(flavor, &s)
	if err != nil {
		return instance, err
	}

	// Merge UserData into Init
	lines := []string{}
	if instance.Init != "" {
		lines = append(lines, instance.Init)
	}
	lines = append(lines, s.UserData...)

	instance.Init = strings.Join(lines, "\n")

	// Add user Labels as tags
	for k, v := range s.Labels {
		if instance.Tags == nil {
			instance.Tags = map[string]string{}
		}
		instance.Tags[k] = v
	}
	return instance, nil
}
