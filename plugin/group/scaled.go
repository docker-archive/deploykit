package group

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

// Scaled is a collection of instances that can be scaled up and down.
type Scaled interface {
	// CreateOne creates a single instance in the scaled group.  Parameters may be provided to customize behavior
	// of the instance.
	CreateOne(privateIP *string, volume *instance.VolumeID)

	// Destroy destroys a single instance.
	Destroy(id instance.ID)

	// List returns all instances in the group.
	List() ([]instance.Description, error)
}

type scaledGroup struct {
	instancePlugin   instance.Plugin
	memberTags       map[string]string
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
	tags[configTag] = settings.config.instanceHash()

	s.provisionTags = tags
}

func (s *scaledGroup) CreateOne(privateIP *string, volume *instance.VolumeID) {

	id, err := s.instancePlugin.Provision(s.provisionRequest, s.provisionTags, privateIP, volume)
	if err != nil {
		log.Errorf("Failed to provision: %s", err)
		return
	}

	volumeDesc := ""
	if volume != nil {
		volumeDesc = fmt.Sprintf(" and volume %s", *volume)
	}

	log.Infof("Created instance %s with tags %v%s", *id, s.provisionTags, volumeDesc)
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
