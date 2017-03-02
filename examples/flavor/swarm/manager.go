package main

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/discovery"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewManagerFlavor(plugins func() discovery.Plugins, connect func(Spec) (client.APIClient, error),
	templ *template.Template,
	stop <-chan struct{}) *ManagerFlavor {

	base := &baseFlavor{initScript: templ, getDockerClient: connect, plugins: plugins}
	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &ManagerFlavor{baseFlavor: base}
}

// ManagerFlavor is the flavor for swarm managers
type ManagerFlavor struct {
	*baseFlavor
}

// Validate checks whether the helper can support a configuration.
func (s *ManagerFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {

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
			log.Warnf("LogicalID %s has no attachments, which is needed for durability", id)
		}
	}
	return nil
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *ManagerFlavor) Prepare(flavorProperties *types.Any,
	instanceSpec instance.Spec, allocation group_types.AllocationMethod) (instance.Spec, error) {
	return s.baseFlavor.prepare("manager", flavorProperties, instanceSpec, allocation)
}
