package group

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

// Scaled is a collection of instances that can be scaled up and down.
type Scaled interface {
	// CreateOne creates a single instance in the scaled group.  Parameters may be provided to customize behavior
	// of the instance.
	CreateOne(privateIP *string)

	// Destroy destroys a single instance.
	Destroy(id instance.ID)

	// List returns all instances in the group.
	List() ([]instance.Description, error)
}

type scaledGroup struct {
	instancePlugin   instance.Plugin
	memberTags       map[string]string
	config           group.Configuration
	provisionHelper  types.ProvisionHelper
	provisionRequest json.RawMessage
	provisionTags    map[string]string
	lock             sync.Mutex
}

func (s *scaledGroup) changeSettings(settings groupSettings) {
	s.lock.Lock()
	s.lock.Unlock()

	s.provisionRequest = settings.config.InstancePluginProperties
	tags := map[string]string{}
	for k, v := range s.memberTags {
		tags[k] = v
	}

	// Instances are tagged with a SHA of the entire instance configuration to support change detection.
	tags[configTag] = settings.config.InstanceHash()

	s.provisionTags = tags
}

func (s *scaledGroup) CreateOne(privateIP *string) {

	spec := instance.Spec{Tags: s.provisionTags, PrivateIPAddress: privateIP}

	if s.provisionHelper != nil {
		// Copy tags to prevent concurrency issues if modified.
		tags := map[string]string{}
		for k, v := range spec.Tags {
			tags[k] = v
		}
		spec.Tags = tags

		var err error
		spec, err = s.provisionHelper.PreProvision(s.config, spec)
		if err != nil {
			log.Errorf("Pre-provision failed: %s", err)
			return
		}
	}

	spec.Properties = s.provisionRequest

	id, err := s.instancePlugin.Provision(spec)
	if err != nil {
		log.Errorf("Failed to provision: %s", err)
		return
	}

	volumeDesc := ""
	if spec.Volume != nil {
		volumeDesc = fmt.Sprintf(" and volume %s", *spec.Volume)
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
