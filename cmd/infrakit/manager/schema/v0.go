package schema

import (
	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// ParseInputSpecs parses the input bytes which is the groups.json, and calls
// each time a group spec is found.
func ParseInputSpecs(input []byte, foundGroupSpec func(plugin.Name, group.ID, group_types.Spec) error) error {
	// TODO - update the schema soon. This is the Plugin/Properties schema
	type spec struct {
		Plugin     plugin.Name
		Properties struct {
			ID         group.ID
			Properties group_types.Spec
		}
	}

	specs := []spec{}
	err := types.AnyBytes(input).Decode(&specs)
	if err != nil {
		return err
	}
	for _, s := range specs {
		err = foundGroupSpec(s.Plugin, s.Properties.ID, s.Properties.Properties)
		if err != nil {
			return err
		}
	}
	return nil
}
