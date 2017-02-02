package group

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"sync"
)

// Scaled is a collection of instances that can be scaled up and down.
type Scaled interface {
	// CreateOne creates a single instance in the scaled group.  Parameters may be provided to customize behavior
	// of the instance.
	CreateOne(id *instance.LogicalID)

	// Health inspects the current health state of an instance.
	Health(inst instance.Description) flavor.Health

	// Destroy destroys a single instance.
	Destroy(inst instance.Description)

	// List returns all instances in the group.
	List() ([]instance.Description, error)
}

type scaledGroup struct {
	settings   groupSettings
	memberTags map[string]string
	lock       sync.Mutex
}

func (s *scaledGroup) changeSettings(settings groupSettings) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.settings = settings
}

// latestSettings gives a point-in-time view of the settings for this group.  This allows other functions to
// safely use settings and make calls to plugins without holding the lock.
func (s *scaledGroup) latestSettings() groupSettings {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.settings
}

func (s *scaledGroup) CreateOne(logicalID *instance.LogicalID) {
	settings := s.latestSettings()

	tags := map[string]string{}
	for k, v := range s.memberTags {
		tags[k] = v
	}

	// Instances are tagged with a SHA of the entire instance configuration to support change detection.
	tags[configTag] = settings.config.InstanceHash()

	spec := instance.Spec{
		Tags:       tags,
		LogicalID:  logicalID,
		Properties: types.AnyCopy(settings.config.Instance.Properties),
	}

	spec, err := settings.flavorPlugin.Prepare(types.AnyCopy(settings.config.Flavor.Properties),
		spec,
		settings.config.Allocation)
	if err != nil {
		log.Errorf("Failed to Prepare instance: %s", err)
		return
	}

	id, err := settings.instancePlugin.Provision(spec)
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

func (s *scaledGroup) Health(inst instance.Description) flavor.Health {
	settings := s.latestSettings()

	health, err := settings.flavorPlugin.Healthy(types.AnyCopy(settings.config.Flavor.Properties), inst)
	if err != nil {
		log.Warnf("Failed to check health of instance %s: %s", inst.ID, err)
		return flavor.Unknown
	}
	return health

}

func (s *scaledGroup) Destroy(inst instance.Description) {
	settings := s.latestSettings()

	flavorProperties := types.AnyCopy(settings.config.Flavor.Properties)
	if err := settings.flavorPlugin.Drain(flavorProperties, inst); err != nil {
		log.Errorf("Failed to drain %s: %s", inst.ID, err)
	}

	log.Infof("Destroying instance %s", inst.ID)
	if err := settings.instancePlugin.Destroy(inst.ID); err != nil {
		log.Errorf("Failed to destroy %s: %s", inst.ID, err)
	}
}

func (s *scaledGroup) List() ([]instance.Description, error) {
	settings := s.latestSettings()

	return settings.instancePlugin.DescribeInstances(s.memberTags)
}
