package local

import (
	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
	group_kind "github.com/docker/infrakit/pkg/run/v0/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// Plugins returns a list of startPlugin directives from the input.
// This will recurse into any composable plugins.
func Plugins(gid group.ID, gspec group_types.Spec) ([]StartPlugin, error) {
	targets := []StartPlugin{}

	spec, err := toSpec(gid, gspec)
	if err != nil {
		return nil, err
	}

	log.Debug("resolving", "groupID", gid, "spec", spec)
	other, err := depends.Resolve(spec, spec.Kind, nil)
	if err != nil {
		return nil, err
	}

	for _, r := range other {
		targets = append(targets, FromAddressable(r))
	}

	return targets, nil
}

func toSpec(gid group.ID, g group_types.Spec) (spec types.Spec, err error) {
	any, e := types.AnyValue(g)
	if e != nil {
		err = e
		return
	}
	spec = types.Spec{
		Kind:    group_kind.Kind,
		Version: group.InterfaceSpec.Encode(),
		Metadata: types.Metadata{
			Identity: &types.Identity{ID: string(gid)},
			Name:     plugin.NameFrom(group_kind.Kind, string(gid)).String(),
		},
		Properties: any,
		Options:    nil, // TOOD -- the old format doesn't have this information.
	}
	return
}
