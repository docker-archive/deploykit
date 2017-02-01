package main

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewManagerFlavor(connect func(Spec) (client.APIClient, error), templ *template.Template) flavor.Plugin {
	return &managerFlavor{&baseFlavor{initScript: templ, getDockerClient: connect}}
}

type managerFlavor struct {
	*baseFlavor
}

func (s *managerFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {

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
func (s *managerFlavor) Prepare(flavorProperties *types.Any,
	instanceSpec instance.Spec, allocation group_types.AllocationMethod) (instance.Spec, error) {
	return s.baseFlavor.prepare("manager", flavorProperties, instanceSpec, allocation)
}
