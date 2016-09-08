package scaler

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"strconv"
	"sync"
	"time"
)

const (
	logicalGroupTag  = "machete.group"
	physicalGroupTag = "machete.generation"
)

// NewGroup creates a new group plugin.
func NewGroup() (group.Plugin, error) {
	return &managedGroup{groups: groups{logical: map[group.ID]*logicalGroup{}}}, nil
}

type managedGroup struct {
	lock   sync.Mutex
	groups groups
}

func getInstancePlugin(properties groupProperties) (instance.Plugin, string, error) {
	// TODO(wfarner): This will change to use plugin discovery once available.
	switch properties.InstancePlugin {
	case "aws":
		return aws.NewPluginFromProperties(properties.InstancePluginProperties)

	case "demo":
		return newDemoPlugin(), "nothing", nil
	default:
		return nil, "", fmt.Errorf("Unsupported Instance plugin '%s'", properties.InstancePlugin)
	}
}

func createScaler(id physicalGroupID, properties groupProperties) (Scaler, error) {
	instancePlugin, instanceRequest, err := getInstancePlugin(properties)
	if err != nil {
		return nil, err
	}

	tags := map[string]string{
		logicalGroupTag:  string(id.gid),
		physicalGroupTag: strconv.Itoa(id.phyID),
	}

	return NewFixedScaler(tags, properties.Size, 5*time.Second, instancePlugin, instanceRequest)
}

func (m *managedGroup) watchPhysicalGroup(id physicalGroupID, properties groupProperties) (*physicalGroup, error) {
	scaler, err := createScaler(id, properties)
	if err != nil {
		return nil, err
	}

	phy := &physicalGroup{properties: &properties, scaler: scaler}
	m.groups.putPhy(id, phy)

	// TODO(wfarner): Consider changing Run() to not block.
	go scaler.Run()
	log.Infof("Watching group '%v'", id)
	return phy, nil
}

func findLatestPhysicalGroup(properties groupProperties, gid group.ID) (int, error) {
	instancePlugin, _, err := getInstancePlugin(properties)
	if err != nil {
		return -1, err
	}

	instances, err := instancePlugin.DescribeInstances(map[string]string{logicalGroupTag: string(gid)})
	if err != nil {
		return -1, err
	}

	highestPhyID := 0
	for _, inst := range instances {
		phyIDString, exists := inst.Tags[physicalGroupTag]
		if exists {
			phyID, err := strconv.Atoi(phyIDString)
			if err == nil {
				log.Info(
					"Found existing instance %s in group %s with physical ID %d",
					inst.ID, gid, phyID)
				if phyID > highestPhyID {
					highestPhyID = phyID
				}
			}

		} else {
			log.Warnf("Found existing instance %s with group tag but no physical ID", inst.ID)
		}
	}

	return highestPhyID, nil
}

func (m *managedGroup) WatchGroup(config group.Configuration) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.groups.get(config.ID); exists {
		return fmt.Errorf("Already watching group '%s'", config.ID)
	}

	properties, err := toProperties(config.Properties)
	if err != nil {
		return err
	}

	phyID, err := findLatestPhysicalGroup(properties, config.ID)
	if err != nil {
		return err
	}

	_, err = m.watchPhysicalGroup(physicalGroupID{gid: config.ID, phyID: phyID}, properties)
	return err
}

func (m *managedGroup) UnwatchGroup(id group.ID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	grp, exists := m.groups.get(id)
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", id)
	}

	for _, phy := range grp.phys {
		phy.scaler.Stop()
	}

	m.groups.deleteLogical(id)
	log.Infof("Stopped watching group '%s'", id)
	return nil
}

func (m *managedGroup) InspectGroup(id group.ID) (group.Description, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	logical, exists := m.groups.get(id)
	if !exists {
		return group.Description{}, fmt.Errorf("Group '%s' is not being watched", id)
	}

	instances := []instance.Description{}
	for _, phy := range logical.phys {
		i, err := phy.scaler.Describe()
		if err != nil {
			return group.Description{}, err
		}

		instances = append(instances, i...)
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

func (m *managedGroup) scalerUpdate(phy *physicalGroup, size uint32) updatePlan {

	if phy.properties.Size == size {
		return updatePlan{desc: "Noop", execute: func() error { return nil }}
	}

	desc := fmt.Sprintf(
		"Changes group size from %d to %d, no restarts necessary",
		phy.properties.Size,
		size)

	execute := func() error {
		m.lock.Lock()
		defer m.lock.Unlock()

		phy.setSize(size)
		return nil
	}

	return updatePlan{desc: desc, execute: execute}
}

const (
	// rollSizeUnchanged refers to a rolling update with no simultaneous change in group size.  No special handling
	// is necessary.
	rollSizeUnchanged int = iota

	// rollSizeDecreased refers to a rolling update where the group size is decreased from N to M.  This is handled
	// by rolling M instances to the new group, followed by terminating the remaining (N-M) instances in the
	// old group.
	rollSizeDecreased

	// rollSizeIncreased refers to a rolling update where the group size is increased from N to M.  This is handled
	// by rolling N instances to the new group, followed by increasing the new group size to M.
	rollSizeIncreased
)

func (m *managedGroup) rollingUpdate(logical *logicalGroup, id physicalGroupID, newProps groupProperties) updatePlan {

	phy, _ := logical.getPhy(id.phyID)

	var rollCount uint32
	var rollType int
	var desc string
	sizeChange := int(newProps.Size) - int(phy.properties.Size)
	switch {
	case sizeChange == 0:
		rollCount = newProps.Size
		rollType = rollSizeUnchanged
		desc = fmt.Sprintf("Performs a rolling update on %d instances", rollCount)
	case sizeChange < 0:
		rollCount = newProps.Size
		rollType = rollSizeDecreased
		desc = fmt.Sprintf(
			"Performs a rolling update on %d instances, "+
				"then terminates %d original instances to reduce the group size to %d",
			rollCount,
			int(sizeChange)*-1,
			newProps.Size)
	case sizeChange > 0:
		rollCount = phy.properties.Size
		rollType = rollSizeIncreased
		desc = fmt.Sprintf(
			"Performs a rolling update on %d instances, "+
				"then adds %d instances to increase the group size to %d",
			rollCount,
			sizeChange,
			newProps.Size)
	}

	execute := func() error {
		m.lock.Lock()
		defer m.lock.Unlock()

		// Store the final desired size and synthetically start the new physical group at size 0.
		finalSize := newProps.Size
		newProps.Size = 0

		newPhysicalID := physicalGroupID{gid: id.gid, phyID: id.phyID + 1}

		newPhy, err := m.watchPhysicalGroup(newPhysicalID, newProps)
		if err != nil {
			return err
		}

		update := rollingupdate{oldGroup: phy.scaler, newGroup: newPhy.scaler, count: rollCount}
		go func() {
			update.Run()

			m.lock.Lock()
			defer m.lock.Unlock()

			switch rollType {
			case rollSizeUnchanged:
				log.Infof("Rolling update of group %v completed", id)
			case rollSizeDecreased:
				log.Infof("Rolling phase of update to group %v completed, removing old instances", id)

			case rollSizeIncreased:
				log.Infof("Rolling phase of update to group %v completed, adding new instances", id)
				newPhy.setSize(finalSize)
			}

			err := m.destroyPhysicalGroupWithLock(logical, id.phyID)
			if err != nil {
				log.Warn(err)
			}
		}()

		return nil
	}

	return updatePlan{desc: desc, execute: execute}
}

func (m *managedGroup) planUpdate(updated group.Configuration) (updatePlan, error) {

	plan := updatePlan{}

	logical, exists := m.groups.get(updated.ID)
	if !exists {
		return plan, fmt.Errorf("Group '%s' is not being watched", updated.ID)
	}

	phyID, phy, err := logical.getOnlyPhy()
	if err != nil {
		// TODO(wfarner): Allow an update to resume if the target group exists and matches `updated`.
		return plan, err
	}

	existingProps := phy.properties
	newProps, err := toProperties(updated.Properties)
	if err != nil {
		return plan, err
	}

	if differsBySizeOnly(*existingProps, newProps) {
		plan = m.scalerUpdate(phy, newProps.Size)
	} else {
		plan = m.rollingUpdate(logical, physicalGroupID{gid: updated.ID, phyID: phyID}, newProps)
	}

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
	plan, err := m.planUpdate(updated)
	if err != nil {
		return err
	}

	log.Infof("Executing update plan for '%s': %s", updated.ID, plan.desc)
	return plan.execute()
}

func (m *managedGroup) destroyPhysicalGroupWithLock(logical *logicalGroup, id int) error {
	log.Infof("Destroying generation %d", id)

	phy, exists := logical.getPhy(id)
	if !exists {
		return fmt.Errorf("Physical group %d does not exist", id)
	}

	phy.scaler.Stop()
	err := phy.scaler.Destroy()
	if err != nil {
		return err
	}

	logical.deletePhy(id)

	log.Infof("Finished destroying group '%d'", id)
	return nil
}

func (m *managedGroup) DestroyGroup(gid group.ID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	logical, exists := m.groups.get(gid)
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", gid)
	}

	for id := range logical.phys {
		err := m.destroyPhysicalGroupWithLock(logical, id)
		if err != nil {
			return err
		}
	}

	m.groups.deleteLogical(gid)

	return nil
}
