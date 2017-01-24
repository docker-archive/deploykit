package main

import (
	"encoding/json"
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
func NewManagerFlavor(dockerClient client.APIClient, templ *template.Template) flavor.Plugin {
	return &managerFlavor{client: dockerClient, initScript: templ}
}

type managerFlavor struct {
	client     client.APIClient
	initScript *template.Template
}

func (s *managerFlavor) Validate(flavorProperties json.RawMessage, allocation group_types.AllocationMethod) error {
	spec := Spec{}
	err := types.AnyBytes([]byte(flavorProperties)).Decode(&spec)
	if err != nil {
		return err
	}

	if spec.InitScriptTemplateURL != "" {
		_, err := template.NewTemplate(spec.InitScriptTemplateURL, defaultTemplateOptions)
		if err != nil {
			return err
		}
	}

	numIDs := len(allocation.LogicalIDs)
	if numIDs != 1 && numIDs != 3 && numIDs != 5 {
		return errors.New("Must have 1, 3, or 5 manager logical IDs")
	}

	for _, id := range allocation.LogicalIDs {
		if att, exists := spec.Attachments[id]; !exists || len(att) == 0 {
			log.Warnf("LogicalID %s has no attachments, which is needed for durability", id)
		}
	}

	if err := validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments); err != nil {
		return err
	}
	return nil
}

// ExportTemplateFunctions returns the functions that are to exported in templates
func (s *managerFlavor) ExportTemplateFunctions() []template.Function {
	swarmStatus, self, err := swarmState(s.client)
	if err != nil {
		return nil
	}
	return exportTemplateFunctions(swarmStatus, self, *types.NewLink())
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s *managerFlavor) Healthy(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
	return healthy(s.client, inst)
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *managerFlavor) Prepare(flavorProperties json.RawMessage,
	instanceSpec instance.Spec, allocation group_types.AllocationMethod) (instance.Spec, error) {

	spec := Spec{}
	any := types.AnyBytes([]byte(flavorProperties))
	err := any.Decode(&spec)
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

	swarmStatus, node, err := swarmState(s.client)
	if err != nil {
		return instanceSpec, err
	}

	link := types.NewLink().WithContext("swarm/" + swarmStatus.ID + "/manager")

	initTemplate.AddFuncs(exportTemplateFunctions(swarmStatus, node, *link))
	initScript, err := initTemplate.Render(nil)
	if err != nil {
		return instanceSpec, err
	}

	log.Infoln("Init script:", initScript)

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

// Drain only explicitly remove worker nodes, not manager nodes.  Manager nodes are assumed to have an
// attached volume for state, and fixed IP addresses.  This allows them to rejoin as the same node.
func (s *managerFlavor) Drain(flavorProperties json.RawMessage, inst instance.Description) error {
	return nil
}
