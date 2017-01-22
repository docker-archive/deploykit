package manager

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
)

// globalSpec is a simple model of a collection of Group plugin configs
type globalSpec struct {

	// Groups is a map keyed by the group.ID which is a nested field inside the spec's Properties.
	// This is not a representation to be used externally because external, user-facing representation
	// should be a list of plugin.Spec and not map to enforce the invariant that all YAML/JSON keys fields are
	// fields in objects.  See https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#lists-of-named-subobjects-preferred-over-maps
	Groups map[group.ID]plugin.Spec
}
