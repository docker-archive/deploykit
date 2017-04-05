package group

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	bootstrapConfigTag = "bootstrap"
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

	// Label makes sure all instances in the group are labelled.
	Label() error
}

type scaledGroup struct {
	supervisor Supervisor
	scaler     *scaler
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

	index := group_types.Index{
		Group:    s.supervisor.ID(),
		Sequence: s.supervisor.Size(),
	}
	spec, err := settings.flavorPlugin.Prepare(types.AnyCopy(settings.config.Flavor.Properties),
		spec,
		settings.config.Allocation,
		index)
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

	return settings.instancePlugin.DescribeInstances(s.memberTags, false)
}

func (s *scaledGroup) Label() error {
	settings := s.latestSettings()

	instances, err := settings.instancePlugin.DescribeInstances(s.memberTags, false)
	if err != nil {
		return err
	}

	tagsWithConfigSha := map[string]string{}
	for k, v := range s.memberTags {
		tagsWithConfigSha[k] = v
	}
	tagsWithConfigSha[configTag] = settings.config.InstanceHash()

	for _, inst := range instances {
		if instanceNeedsLabel(inst) {
			log.Infof("Labelling instance %s", inst.ID)

			if err := settings.instancePlugin.Label(inst.ID, tagsWithConfigSha); err != nil {
				return err
			}
		}
	}

	return nil
}

func labelAndList(scaled Scaled) ([]instance.Description, error) {
	descriptions, err := scaled.List()
	if err != nil {
		return nil, err
	}

	if !needsLabel(descriptions) {
		return descriptions, nil
	}

	if err := scaled.Label(); err != nil {
		return nil, err
	}

	return scaled.List()
}

func needsLabel(instances []instance.Description) bool {
	for _, inst := range instances {
		if instanceNeedsLabel(inst) {
			return true
		}
	}

	return false
}

func instanceNeedsLabel(instance instance.Description) bool {
	return instance.Tags[configTag] == bootstrapConfigTag
}
