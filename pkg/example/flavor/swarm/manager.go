package main

import (
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/plugin/group/util"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"golang.org/x/net/context"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewManagerFlavor(dockerClient client.APIClient, templ *template.Template) flavor.Plugin {
	return &managerFlavor{client: dockerClient, initScript: templ}
}

type managerFlavor struct {
	client     client.APIClient
	initScript *template.Template
}

func (s *managerFlavor) Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return err
	}

	if properties.DockerRestartCommand == "" {
		return errors.New("DockerRestartCommand must be specified")
	}

	numIDs := len(allocation.LogicalIDs)
	if numIDs != 1 && numIDs != 3 && numIDs != 5 {
		return errors.New("Must have 1, 3, or 5 manager logical IDs")
	}

	for _, id := range allocation.LogicalIDs {
		if att, exists := properties.Attachments[id]; !exists || len(att) == 0 {
			log.Warnf("LogicalID %s has no attachments, which is needed for durability", id)
		}
	}

	if err := validateIDsAndAttachments(allocation.LogicalIDs, properties.Attachments); err != nil {
		return err
	}
	return nil
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s *managerFlavor) Healthy(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
	return healthy(s.client, flavorProperties, inst)
}

func (s *managerFlavor) Prepare(flavorProperties json.RawMessage,
	spec instance.Spec, allocation types.AllocationMethod) (instance.Spec, error) {

	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return spec, err
	}

	swarmStatus, err := s.client.SwarmInspect(context.Background())
	if err != nil {
		return spec, fmt.Errorf("Failed to fetch Swarm join tokens: %s", err)
	}

	nodeInfo, err := s.client.Info(context.Background())
	if err != nil {
		return spec, fmt.Errorf("Failed to fetch node self info: %s", err)
	}

	self, _, err := s.client.NodeInspectWithRaw(context.Background(), nodeInfo.Swarm.NodeID)
	if err != nil {
		return spec, fmt.Errorf("Failed to fetch Swarm node status: %s", err)
	}

	if self.ManagerStatus == nil {
		return spec, errors.New(
			"Swarm node status did not include manager status.  Need to run 'docker swarm init`?")
	}

	associationID := util.RandomAlphaNumericString(8)
	spec.Tags[associationTag] = associationID

	if spec.LogicalID == nil {
		return spec, errors.New("Manager nodes require a LogicalID, " +
			"which will be used as an assigned private IP address")
	}

	initScript, err := generateInitScript(
		s.initScript,
		self.ManagerStatus.Addr,
		swarmStatus.JoinTokens.Manager,
		associationID,
		properties.DockerRestartCommand)
	if err != nil {
		return spec, err
	}
	spec.Init = initScript

	if spec.LogicalID != nil {
		if attachments, exists := properties.Attachments[*spec.LogicalID]; exists {
			spec.Attachments = append(spec.Attachments, attachments...)
		}
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	spec.Tags["swarm-id"] = swarmStatus.ID

	return spec, nil
}

// Drain only explicitly remove worker nodes, not manager nodes.  Manager nodes are assumed to have an
// attached volume for state, and fixed IP addresses.  This allows them to rejoin as the same node.
func (s *managerFlavor) Drain(flavorProperties json.RawMessage, inst instance.Description) error {
	return nil
}
