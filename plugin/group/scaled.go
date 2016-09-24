package group

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

// Scaled is a collection of instances that can be scaled up and down.
type Scaled interface {
	// CreateOne creates a single instance in the scaled group.  Parameters may be provided to customize behavior
	// of the instance.
	CreateOne(id *instance.LogicalID)

	// Destroy destroys a single instance.
	Destroy(id instance.ID)

	// List returns all instances in the group.
	List() ([]instance.Description, error)
}

type scaledGroup struct {
	instancePlugin   instance.Plugin
	memberTags       map[string]string
	flavorPlugin     flavor.Plugin
	flavorProperties json.RawMessage
	provisionRequest json.RawMessage
	provisionTags    map[string]string
	lock             sync.Mutex
}

func (s *scaledGroup) changeSettings(settings groupSettings) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.provisionRequest = settings.config.InstancePluginProperties
	tags := map[string]string{}
	for k, v := range s.memberTags {
		tags[k] = v
	}

	// Instances are tagged with a SHA of the entire instance configuration to support change detection.
	tags[configTag] = settings.config.InstanceHash()

	s.provisionTags = tags
}

func (s *scaledGroup) getSpec(logicalID *instance.LogicalID) (instance.Spec, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	spec := instance.Spec{
		Tags:       s.provisionTags,
		LogicalID:  logicalID,
		Properties: s.provisionRequest,
	}

	if s.flavorPlugin != nil {
		// Copy tags to prevent concurrency issues if modified.
		tags := map[string]string{}
		for k, v := range spec.Tags {
			tags[k] = v
		}
		spec.Tags = tags

		var err error
		spec, err = s.flavorPlugin.PreProvision(s.flavorProperties, spec)
		if err != nil {
			return spec, err
		}
	}

	return spec, nil
}

func (s *scaledGroup) CreateOne(logicalID *instance.LogicalID) {
	spec, err := s.getSpec(logicalID)
	if err != nil {
		log.Errorf("Pre-provision failed: %s", err)
		return
	}

	id, err := s.instancePlugin.Provision(spec)
	if err != nil {
		log.Errorf("Failed to provision: %s", err)
		return
	}

	volumeDesc := ""
	if len(spec.Attachments) > 0 {
		volumeDesc = fmt.Sprintf(" and attachments %s", spec.Attachments)
	}

	log.Infof("Created instance %s with tags %v%s", *id, spec.Tags, volumeDesc)
}

func (s *scaledGroup) Destroy(id instance.ID) {
	log.Infof("Destroying instance %s", id)
	if err := s.instancePlugin.Destroy(id); err != nil {
		log.Errorf("Failed to destroy %s: %s", id, err)
	}
}

func (s *scaledGroup) List() ([]instance.Description, error) {
	return s.instancePlugin.DescribeInstances(s.memberTags)
}
