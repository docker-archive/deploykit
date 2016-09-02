package scaler

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/spi/group"
	"sync"
	"time"
)

// NewGroup creates a new group plugin.
func NewGroup() (group.Plugin, error) {
	return &managedGroup{groups: make(map[group.ID]groupContext)}, nil
}

type managedGroup struct {
	lock   sync.Mutex
	groups map[group.ID]groupContext
}

func createScaler(id group.ID, properties groupProperties) (Scaler, error) {
	// TODO(wfarner): This will change to use plugin discovery once available.
	if properties.InstancePlugin != "aws" {
		return nil, fmt.Errorf("Unsupported Instance plugin '%s'", properties.InstancePlugin)
	}

	instancePlugin, instancePluginRequest, err := aws.NewPluginFromProperties(properties.InstancePluginProperties)
	if err != nil {
		return nil, err
	}

	return NewFixedScaler(id, properties.Size, 5*time.Second, instancePlugin, instancePluginRequest)
}

func (m *managedGroup) WatchGroup(config group.Configuration) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.groups[config.ID]; exists {
		return fmt.Errorf("Already watching group '%s'", config.ID)
	}

	properties, err := toProperties(config.Properties)
	if err != nil {
		return err
	}

	scaler, err := createScaler(config.ID, properties)
	if err != nil {
		return err
	}

	m.groups[config.ID] = groupContext{properties: properties, scaler: scaler}

	// TODO(wfarner): Consider changing Run() to not block.
	go m.groups[config.ID].scaler.Run()
	log.Infof("Watching group '%s'", config.ID)
	return nil
}

func (m *managedGroup) UnwatchGroup(id group.ID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	grp, exists := m.groups[id]
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", id)
	}

	grp.scaler.Stop()
	delete(m.groups, id)
	log.Infof("Stopped watching group '%s'", id)
	return nil
}

func (m *managedGroup) InspectGroup(id group.ID) (group.Description, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, exists := m.groups[id]
	if !exists {
		return group.Description{}, fmt.Errorf("Group '%s' is not being watched", id)
	}

	return group.Description{}, nil
}

func toProperties(properties json.RawMessage) (groupProperties, error) {
	props := groupProperties{}
	err := json.Unmarshal([]byte(properties), &props)
	if err != nil {
		err = fmt.Errorf("Invalid properties: %s", err)
	}

	return props, err
}

func differsBySizeOnly(a groupProperties, b groupProperties) bool {
	a.Size = 0
	b.Size = 0
	// TODO(wfarner): Hack.
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

type updatePlan struct {
	desc    string
	execute func() error
}

func (m *managedGroup) targetCountAdjustment(
	id group.ID,
	oldContext groupContext,
	newProperties groupProperties) func() error {

	return func() error {
		scaler, err := createScaler(id, newProperties)
		if err != nil {
			return err
		}

		oldContext.scaler.Stop()
		newContext := groupContext{properties: newProperties, scaler: scaler}
		m.groups[id] = newContext
		go newContext.scaler.Run()
		return nil
	}
}

func (m *managedGroup) planUpdate(updated group.Configuration) (updatePlan, error) {
	plan := updatePlan{}

	existing, exists := m.groups[updated.ID]
	if !exists {
		return plan, fmt.Errorf("Group '%s' is not being watched", updated.ID)
	}

	existingProps := existing.properties
	newProps, err := toProperties(updated.Properties)
	if err != nil {
		return plan, err
	}

	if !differsBySizeOnly(existingProps, newProps) {
		return plan, errors.New("Proposed update is not supported")
	}

	plan.desc = fmt.Sprintf(
		"Changes group Size from %d to %d, no restarts necessary",
		existingProps.Size,
		newProps.Size)
	plan.execute = m.targetCountAdjustment(updated.ID, existing, newProps)

	return plan, nil
}

func (m *managedGroup) DescribeUpdate(updated group.Configuration) (string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	plan, err := m.planUpdate(updated)
	if err != nil {
		return "", err
	}

	return plan.desc, nil
}

func (m *managedGroup) UpdateGroup(updated group.Configuration) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	plan, err := m.planUpdate(updated)
	if err != nil {
		return err
	}

	log.Infof("Executing update plan for '%s'", updated.ID)
	return plan.execute()
}

func (m *managedGroup) DestroyGroup(grp group.ID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	toDestroy, exists := m.groups[grp]
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", grp)
	}

	log.Infof("Destroying group %s", grp)
	toDestroy.scaler.Stop()
	err := toDestroy.scaler.Destroy()
	if err != nil {
		return err
	}

	delete(m.groups, grp)
	log.Infof("Finished destroying group '%s'", grp)
	return nil
}

type groupProperties struct {
	Size                     uint
	InstancePlugin           string
	InstancePluginProperties json.RawMessage
}

type groupContext struct {
	properties groupProperties
	scaler     Scaler
}
