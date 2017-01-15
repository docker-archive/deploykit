package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"golang.org/x/net/context"
)

const (
	ebsAttachment string = "ebs"
)

type swarmFlavor struct {
	client     client.APIClient
	initScript *template.Template
}

type schema struct {
	Attachments          map[instance.LogicalID][]instance.Attachment
	DockerRestartCommand string
}

func parseProperties(flavorProperties json.RawMessage) (schema, error) {
	s := schema{}
	err := json.Unmarshal(flavorProperties, &s)
	return s, err
}

func validateIDsAndAttachments(logicalIDs []instance.LogicalID,
	attachments map[instance.LogicalID][]instance.Attachment) error {

	// Each attachment association must be represented by a logical ID.
	idsMap := map[instance.LogicalID]bool{}
	for _, id := range logicalIDs {
		if _, exists := idsMap[id]; exists {
			return fmt.Errorf("LogicalID %v specified more than once", id)
		}

		idsMap[id] = true
	}
	for id := range attachments {
		if _, exists := idsMap[id]; !exists {
			return fmt.Errorf("LogicalID %v used for an attachment but is not in group LogicalIDs", id)
		}
	}

	// Only EBS attachments are supported.
	for _, atts := range attachments {
		for _, attachment := range atts {
			if attachment.Type == "" {
				return fmt.Errorf(
					"Attachment Type %s must be specified for '%s'",
					ebsAttachment,
					attachment.ID)
			}

			if attachment.Type != ebsAttachment {
				return fmt.Errorf(
					"Invalid attachment Type '%s', only %s is supported",
					attachment.Type,
					ebsAttachment)
			}
		}
	}

	// Each attachment may only be used once.
	allAttachmentIDs := map[string]bool{}
	for _, atts := range attachments {
		for _, attachment := range atts {
			if _, exists := allAttachmentIDs[attachment.ID]; exists {
				return fmt.Errorf("Attachment %v specified more than once", attachment.ID)
			}
			allAttachmentIDs[attachment.ID] = true
		}
	}

	return nil
}

const (
	// associationTag is a machine tag added to associate machines with Swarm nodes.
	associationTag = "swarm-association-id"
)

func generateInitScript(templ *template.Template,
	joinIP, joinToken, associationID, restartCommand string) (string, error) {

	var buffer bytes.Buffer
	err := templ.Execute(&buffer, map[string]string{
		"MY_IP":          joinIP,
		"JOIN_TOKEN":     joinToken,
		"ASSOCIATION_ID": associationID,
		"RESTART_DOCKER": restartCommand,
	})
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (s swarmFlavor) Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
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
func healthy(client client.APIClient,
	flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {

	associationID, exists := inst.Tags[associationTag]
	if !exists {
		log.Info("Reporting unhealthy for instance without an association tag", inst.ID)
		return flavor.Unhealthy, nil
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", associationTag, associationID))

	nodes, err := client.NodeList(context.Background(), docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return flavor.Unknown, err
	}

	switch {
	case len(nodes) == 0:
		// The instance may not yet be joined, so we consider the health unknown.
		return flavor.Unknown, nil

	case len(nodes) == 1:
		return flavor.Healthy, nil

	default:
		log.Warnf("Expected at most one node with label %s, but found %s", associationID, nodes)
		return flavor.Healthy, nil
	}
}
