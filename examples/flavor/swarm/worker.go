package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

// NewWorkerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewWorkerFlavor(connect func(Spec) (client.APIClient, error), templ *template.Template) flavor.Plugin {
	return &workerFlavor{initScript: templ, getDockerClient: connect}
}

type workerFlavor struct {
	getDockerClient func(Spec) (client.APIClient, error)
	initScript      *template.Template
}

func (s *workerFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {
	if flavorProperties == nil {
		return fmt.Errorf("missing config")
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)

	if err != nil {
		return err
	}

	if spec.Docker.Host == "" && spec.Docker.TLS == nil {
		return fmt.Errorf("no docker connect info")
	}

	if spec.InitScriptTemplateURL != "" {
		_, err := template.NewTemplate(spec.InitScriptTemplateURL, defaultTemplateOptions)
		if err != nil {
			return err
		}
	}

	if err := validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments); err != nil {
		return err
	}

	return nil
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s *workerFlavor) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	if flavorProperties == nil {
		return flavor.Unknown, fmt.Errorf("missing config")
	}
	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return flavor.Unknown, err
	}
	dockerClient, err := s.getDockerClient(spec)
	if err != nil {
		return flavor.Unknown, err
	}

	return healthy(dockerClient, inst)
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *workerFlavor) Prepare(flavorProperties *types.Any, instanceSpec instance.Spec,
	allocation group_types.AllocationMethod) (instance.Spec, error) {

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return instanceSpec, err
	}

	initTemplate := s.initScript

	if spec.InitScriptTemplateURL != "" {

		t, err := template.NewTemplate(spec.InitScriptTemplateURL, defaultTemplateOptions)
		if err != nil {
			return instanceSpec, err
		}

		initTemplate = t
		log.Infoln("Using", spec.InitScriptTemplateURL, "for init script template")
	}

	var swarmID, initScript string
	var swarmStatus *swarm.Swarm
	var node *swarm.Node
	var link *types.Link

	for i := 0; ; i++ {
		log.Infoln("WORKER >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>", i, "querying docker swarm")

		dockerClient, err := s.getDockerClient(spec)
		if err != nil {
			log.Warningln("Cannot connect to Docker:", err)
			continue
		}

		swarmStatus, node, err = swarmState(dockerClient)
		if err != nil {
			log.Warningln("Worker prepare:", err)
		}

		swarmID := "?"
		if swarmStatus != nil {
			swarmID = swarmStatus.ID
		}

		link = types.NewLink().WithContext("swarm/" + swarmID + "/worker")
		context := &templateContext{
			flavorSpec:   spec,
			instanceSpec: instanceSpec,
			allocation:   allocation,
			swarmStatus:  swarmStatus,
			nodeInfo:     node,
			link:         *link,
		}
		initScript, err = initTemplate.Render(context)

		log.Infoln("WORKER >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> context.retries =", context.retries, "err=", err, "i=", i)

		if err == nil {
			break
		} else {
			if context.retries == 0 || i == context.retries {
				log.Warningln("Retries exceeded and error:", err)
				return instanceSpec, err
			}

			log.Infoln("Going to wait for swarm to be ready. i=", i)
			time.Sleep(context.poll)
		}
	}

	log.Infoln("WORKER Init script:", initScript)

	instanceSpec.Init = initScript

	if instanceSpec.LogicalID != nil {
		if attachments, exists := spec.Attachments[*instanceSpec.LogicalID]; exists {
			instanceSpec.Attachments = append(instanceSpec.Attachments, attachments...)
		}
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	instanceSpec.Tags["swarm-id"] = swarmID
	link.WriteMap(instanceSpec.Tags)

	return instanceSpec, nil
}

func (s *workerFlavor) Drain(flavorProperties *types.Any, inst instance.Description) error {
	if flavorProperties == nil {
		return fmt.Errorf("missing config")
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}

	dockerClient, err := s.getDockerClient(spec)
	if err != nil {
		return err
	}

	link := types.NewLinkFromMap(inst.Tags)
	if !link.Valid() {
		return fmt.Errorf("Unable to drain %s without an association tag", inst.ID)
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", link.Label(), link.Value()))

	nodes, err := dockerClient.NodeList(context.Background(), docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return err
	}

	switch {
	case len(nodes) == 0:
		return fmt.Errorf("Unable to drain %s, not found in swarm", inst.ID)

	case len(nodes) == 1:
		err := dockerClient.NodeRemove(
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
