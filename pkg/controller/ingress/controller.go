package ingress

import (
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/types"
)

// NewController returns a controller implementation
func NewController(leader manager.Leadership) controller.Controller {
	return internal.NewController(
		leader,
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return nil, nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)
}
