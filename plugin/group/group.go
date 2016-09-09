package scaler

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sync"
	"time"
)

const (
	groupTag  = "machete.group"
	configTag = "machete.config_sha"
)

// NewGroup creates a new group plugin.
func NewGroup(
	plugins map[string]instance.Plugin,
	pollInterval time.Duration) group.Plugin {

	return &managedGroup{
		plugins:      plugins,
		pollInterval: pollInterval,
		groups:       groups{contexts: map[group.ID]*groupContext{}},
	}
}

type managedGroup struct {
	plugins      map[string]instance.Plugin
	pollInterval time.Duration
	lock         sync.Mutex
	groups       groups
}

func instanceConfigHash(instanceProperties json.RawMessage) string {
	hasher := sha1.New()
	hasher.Write(instanceProperties)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func identityTags(properties groupProperties) map[string]string {
	// Instances are tagged with a SHA of the entire instance configuration to support change detection.
	return map[string]string{configTag: instanceConfigHash(properties.InstancePluginProperties)}
}

func (m *managedGroup) WatchGroup(config group.Configuration) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if config.ID == "" {
		return errors.New("Group ID must not be blank")
	}

	if _, exists := m.groups.get(config.ID); exists {
		return fmt.Errorf("Already watching group '%s'", config.ID)
	}

	properties, err := toProperties(config.Properties)
	if err != nil {
		return err
	}

	instancePlugin, exists := m.plugins[properties.InstancePlugin]
	if !exists {
		return fmt.Errorf("Instance plugin '%s' is not available", properties.InstancePlugin)
	}

	err = instancePlugin.Validate(properties.InstancePluginProperties)
	if err != nil {
		return err
	}

	// Two sets of instance tags are used - one for defining membership within the group, and another used to tag
	// newly-created instances.  This allows the scaler to collect and report members of a group which have
	// membership tags but different generation-specific tags.  In practice, we use this the additional tags to
	// attach a config SHA to instances for config change detection.
	scaled := &scaledGroup{
		instancePlugin: instancePlugin,
		// TODO(wfarner): Members will also need to be tagged with the Swarm cluster UUID.
		memberTags:       map[string]string{groupTag: string(config.ID)},
		provisionRequest: properties.InstancePluginProperties,
	}
	scaled.setAdditionalTags(identityTags(properties))

	scaler := NewAdjustableScaler(scaled, properties.Size, m.pollInterval)

	m.groups.put(config.ID, &groupContext{
		properties:     &properties,
		instancePlugin: instancePlugin,
		scaler:         scaler,
		scaled:         scaled})

	// TODO(wfarner): Consider changing Run() to not block.
	go scaler.Run()
	log.Infof("Watching group '%v'", config.ID)

	return nil
}

func (m *managedGroup) UnwatchGroup(id group.ID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	grp, exists := m.groups.get(id)
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", id)
	}

	grp.scaler.Stop()

	m.groups.del(id)
	log.Infof("Stopped watching group '%s'", id)
	return nil
}

func (m *managedGroup) InspectGroup(id group.ID) (group.Description, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	context, exists := m.groups.get(id)
	if !exists {
		return group.Description{}, fmt.Errorf("Group '%s' is not being watched", id)
	}

	instances, err := context.scaled.describe()
	if err != nil {
		return group.Description{}, err
	}

	return group.Description{Instances: instances}, nil
}

func toProperties(properties json.RawMessage) (groupProperties, error) {
	props := groupProperties{}
	err := json.Unmarshal([]byte(properties), &props)
	if err != nil {
		err = fmt.Errorf("Invalid properties: %s", err)
	}

	return props, err
}

type updatePlan struct {
	desc    string
	execute func() error
}

func (m *managedGroup) getContext(id group.ID) (*groupContext, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	context, exists := m.groups.get(id)
	return context, exists
}

func (m *managedGroup) planUpdate(updated group.Configuration) (updatePlan, error) {
	plan := updatePlan{}

	context, exists := m.getContext(updated.ID)
	if !exists {
		return plan, fmt.Errorf("Group '%s' is not being watched", updated.ID)
	}

	newProps, err := toProperties(updated.Properties)
	if err != nil {
		return plan, err
	}

	err = context.instancePlugin.Validate(newProps.InstancePluginProperties)
	if err != nil {
		return plan, err
	}

	return planRollingUpdate(updated.ID, context, newProps)
}

func (m *managedGroup) DescribeUpdate(updated group.Configuration) (string, error) {
	plan, err := m.planUpdate(updated)
	if err != nil {
		return "", err
	}

	return plan.desc, nil
}

func (m *managedGroup) UpdateGroup(updated group.Configuration) error {
	plan, err := m.planUpdate(updated)
	if err != nil {
		return err
	}

	log.Infof("Executing update plan for '%s': %s", updated.ID, plan.desc)
	// TODO(wfarner): While an update is in progress, lock the group to ensure other operations do not interfere.
	return plan.execute()
}

func (m *managedGroup) DestroyGroup(gid group.ID) error {
	m.lock.Lock()

	context, exists := m.groups.get(gid)
	if !exists {
		m.lock.Unlock()
		return fmt.Errorf("Group '%s' is not being watched", gid)
	}

	// The lock is released before performing blocking operations.
	m.groups.del(gid)
	m.lock.Unlock()

	context.scaler.Stop()
	ids, err := context.scaled.List()
	if err != nil {
		return err
	}

	for _, id := range ids {
		err = context.instancePlugin.Destroy(id)
		if err != nil {
			return err
		}
	}

	return nil
}
