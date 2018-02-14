package group

import (
	"fmt"
	"sync"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
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
	Destroy(inst instance.Description, ctx instance.Context) error

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

	if logicalID != nil {
		tags[instance.LogicalIDTag] = string(*logicalID)
	}

	// Instances are tagged with a SHA of the entire instance configuration to support change detection.
	tags[group.ConfigSHATag] = settings.config.InstanceHash()

	spec := instance.Spec{
		Tags:       tags,
		LogicalID:  logicalID,
		Properties: types.AnyCopy(settings.config.Instance.Properties),
	}

	index := group.Index{
		Group:    s.supervisor.ID(),
		Sequence: s.supervisor.Size(),
	}
	spec, err := settings.flavorPlugin.Prepare(types.AnyCopy(settings.config.Flavor.Properties),
		spec,
		settings.config.Allocation,
		index)
	if err != nil {
		log.Error("Failed to Prepare instance", "settings", settings, "err", err)
		return
	}

	id, err := settings.instancePlugin.Provision(spec)
	if err != nil {
		log.Error("Failed to provision", "settings", settings, "err", err)
		return
	}

	volumeDesc := ""
	if len(spec.Attachments) > 0 {
		volumeDesc = fmt.Sprintf(" and attachments %s", spec.Attachments)
	}

	log.Info("Created instance", "id", *id, "tags", spec.Tags, "volumeDesc", volumeDesc)
}

func (s *scaledGroup) Health(inst instance.Description) flavor.Health {
	settings := s.latestSettings()

	health, err := settings.flavorPlugin.Healthy(types.AnyCopy(settings.config.Flavor.Properties), inst)
	if err != nil {
		log.Warn("Failed to check health of instance", "id", inst.ID, "err", err)
		return flavor.Unknown
	}
	return health

}

func (s *scaledGroup) Destroy(inst instance.Description, ctx instance.Context) error {
	settings := s.latestSettings()

	if ctx == instance.RollingUpdate && s.isSkipDrain() {
		log.Info("Skipping drain before instance destroy", "id", inst.ID)
	} else {
		flavorProperties := types.AnyCopy(settings.config.Flavor.Properties)
		log.Info("Draining instance", "id", inst.ID)
		if err := settings.flavorPlugin.Drain(flavorProperties, inst); err != nil {
			// Only error out on a rolling update
			if ctx == instance.RollingUpdate {
				log.Error("Failed to drain", "id", inst.ID, "err", err)
				return err
			}
			log.Warn("Failed to drain, processing with termination", "id", inst.ID, "err", err)
		}
	}

	// Do not destroy the current VM during a rolling update
	if ctx == instance.RollingUpdate && isSelf(inst, s.settings) {
		log.Info("Not destroying self", "LogicalID", *inst.LogicalID)
		return nil
	}
	log.Info("Destroying instance", "id", inst.ID)
	if err := settings.instancePlugin.Destroy(inst.ID, ctx); err != nil {
		log.Error("Failed to destroy instance", "id", inst.ID, "err", err)
		return err
	}
	return nil
}

// returns true if the config is set to skip Drain prior to instance Destroy during
// a rolling update
func (s *scaledGroup) isSkipDrain() bool {
	if s.settings.config.Updating.SkipBeforeInstanceDestroy == nil {
		return false
	}
	return group_types.SkipBeforeInstanceDestroyDrain == *s.settings.config.Updating.SkipBeforeInstanceDestroy
}

func (s *scaledGroup) List() ([]instance.Description, error) {
	settings := s.latestSettings()

	list := []instance.Description{}

	found, err := settings.instancePlugin.DescribeInstances(s.memberTags, true)
	if err != nil {
		return list, err
	}

	// normalize the data. we make sure if there are logical ID in the labels,
	// we also have the LogicalID field populated.
	for _, d := range found {

		// Is there a tag for the logical ID and the logicalID field is not set?
		if logicalIDString, has := d.Tags[instance.LogicalIDTag]; has && d.LogicalID == nil {
			logicalID := instance.LogicalID(logicalIDString)
			d.LogicalID = &logicalID
		}

		list = append(list, d)
	}
	return list, nil
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
	tagsWithConfigSha[group.ConfigSHATag] = settings.config.InstanceHash()

	for _, inst := range instances {
		if instanceNeedsLabel(inst) {
			log.Info("Labelling instance", "id", inst.ID)

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
	return instance.Tags[group.ConfigSHATag] == bootstrapConfigTag
}
