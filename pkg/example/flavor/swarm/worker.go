package main

import (
	"encoding/json"
	"errors"
	"fmt"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/plugin/group/util"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"golang.org/x/net/context"
)

// NewWorkerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewWorkerFlavor(dockerClient client.APIClient, templ *template.Template) flavor.Plugin {
	return &workerFlavor{client: dockerClient, initScript: templ}
}

type workerFlavor struct {
	client     client.APIClient
	initScript *template.Template
}

func (s *workerFlavor) Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return err
	}

	if properties.DockerRestartCommand == "" {
		return errors.New("DockerRestartCommand must be specified")
	}

	if err := validateIDsAndAttachments(allocation.LogicalIDs, properties.Attachments); err != nil {
		return err
	}

	return nil
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s *workerFlavor) Healthy(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
	return healthy(s.client, flavorProperties, inst)
}

func (s *workerFlavor) Prepare(flavorProperties json.RawMessage, spec instance.Spec,
	allocation types.AllocationMethod) (instance.Spec, error) {

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

	initScript, err :=
		generateInitScript(
			s.initScript,
			self.ManagerStatus.Addr,
			swarmStatus.JoinTokens.Worker,
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

func (s *workerFlavor) Drain(flavorProperties json.RawMessage, inst instance.Description) error {

	associationID, exists := inst.Tags[associationTag]
	if !exists {
		return fmt.Errorf("Unable to drain %s without an association tag", inst.ID)
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", associationTag, associationID))

	nodes, err := s.client.NodeList(context.Background(), docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return err
	}

	switch {
	case len(nodes) == 0:
		return fmt.Errorf("Unable to drain %s, not found in swarm", inst.ID)

	case len(nodes) == 1:
		err := s.client.NodeRemove(
			context.Background(),
			nodes[0].ID,
			docker_types.NodeRemoveOptions{Force: true})
		if err != nil {
			return err
		}

		return nil

	default:
		return fmt.Errorf("Expected at most one node with label %s, but found %s", associationID, nodes)
	}
}
