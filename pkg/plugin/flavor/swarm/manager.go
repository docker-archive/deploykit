package swarm

import (
	"errors"

	"github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewManagerFlavor(scope scope.Scope, connect func(Spec) (docker.APIClientCloser, error),
	templ *template.Template,
	stop <-chan struct{}) *ManagerFlavor {

	base := &baseFlavor{initScript: templ, getDockerClient: connect, scope: scope}
	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &ManagerFlavor{baseFlavor: base}
}

// ManagerFlavor is the flavor for swarm managers
type ManagerFlavor struct {
	*baseFlavor
}

// Validate checks whether the helper can support a configuration.
func (s *ManagerFlavor) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {

	if err := s.baseFlavor.Validate(flavorProperties, allocation); err != nil {
		return err
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}

	if len(allocation.LogicalIDs)%2 == 0 {
		return errors.New("must have odd number for quorum")
	}

	for _, id := range allocation.LogicalIDs {
		if att, exists := spec.Attachments[id]; !exists || len(att) == 0 {
			log.Warn("No attachments, which is needed for durability", "id", id)
		}
	}
	return nil
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *ManagerFlavor) Prepare(flavorProperties *types.Any,
	instanceSpec instance.Spec, allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("manager", flavorProperties, instanceSpec, allocation, index)
}
