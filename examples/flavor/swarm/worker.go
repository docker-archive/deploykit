package main

import (
	"encoding/json"
	"fmt"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
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

func (s *workerFlavor) Validate(flavorProperties json.RawMessage, allocation group_types.AllocationMethod) error {
	spec := Spec{}
	err := types.AnyBytes([]byte(flavorProperties)).Decode(&spec)
	if err != nil {
		return err
	}

	if err := validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments); err != nil {
		return err
	}

	return nil
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s *workerFlavor) Healthy(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
	return healthy(s.client, inst)
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *workerFlavor) Prepare(flavorProperties json.RawMessage, instanceSpec instance.Spec,
	allocation group_types.AllocationMethod) (instance.Spec, error) {

	spec := Spec{}
	err := types.AnyBytes([]byte(flavorProperties)).Decode(&spec)
	if err != nil {
		return instanceSpec, err
	}

	swarmStatus, node, err := swarmState(s.client)
	if err != nil {
		return instanceSpec, err
	}

	link := types.NewLink().WithContext("swarm/" + swarmStatus.ID + "/worker")

	s.initScript.AddFuncs(exportTemplateFunctions(swarmStatus, node, *link))
	initScript, err := s.initScript.Render(nil)
	if err != nil {
		return instanceSpec, err
	}

	instanceSpec.Init = initScript

	if instanceSpec.LogicalID != nil {
		if attachments, exists := spec.Attachments[*instanceSpec.LogicalID]; exists {
			instanceSpec.Attachments = append(instanceSpec.Attachments, attachments...)
		}
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	instanceSpec.Tags["swarm-id"] = swarmStatus.ID
	link.WriteMap(instanceSpec.Tags)

	return instanceSpec, nil
}

func (s *workerFlavor) Drain(flavorProperties json.RawMessage, inst instance.Description) error {

	link := types.NewLinkFromMap(inst.Tags)
	if !link.Valid() {
		return fmt.Errorf("Unable to drain %s without an association tag", inst.ID)
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", link.Label(), link.Value()))

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
		return fmt.Errorf("Expected at most one node with label %s, but found %s", link.Value(), nodes)
	}
}
