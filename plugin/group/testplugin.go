package group

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/libmachete/plugin/group/util"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

type fakeInstance struct {
	logicalID *instance.LogicalID
	tags      map[string]string
}

// NewTestInstancePlugin creates a new instance plugin for use in testing and development.
func NewTestInstancePlugin(seedInstances ...fakeInstance) instance.Plugin {
	plugin := testplugin{idPrefix: util.RandomAlphaNumericString(4), instances: map[instance.ID]fakeInstance{}}
	for _, inst := range seedInstances {
		plugin.addInstance(inst)
	}

	return &plugin
}

type testplugin struct {
	lock      sync.Mutex
	idPrefix  string
	nextID    int
	instances map[instance.ID]fakeInstance
}

func (d *testplugin) Validate(req json.RawMessage) error {
	return nil
}

func (d *testplugin) addInstance(inst fakeInstance) instance.ID {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.nextID++
	id := instance.ID(fmt.Sprintf("%s-%d", d.idPrefix, d.nextID))
	d.instances[id] = inst
	return id
}

func (d *testplugin) Provision(spec instance.Spec) (*instance.ID, error) {

	id := d.addInstance(fakeInstance{logicalID: spec.LogicalID, tags: spec.Tags})
	return &id, nil
}

func (d *testplugin) Destroy(id instance.ID) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	_, exists := d.instances[id]
	if !exists {
		return errors.New("Instance does not exist")
	}

	delete(d.instances, id)
	return nil
}

func (d *testplugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	desc := []instance.Description{}
	for id, inst := range d.instances {
		allMatched := true
		for searchKey, searchValue := range tags {
			tagValue, has := inst.tags[searchKey]
			if !has || searchValue != tagValue {
				allMatched = false
			}
		}
		if allMatched {
			desc = append(desc, instance.Description{
				ID:        id,
				LogicalID: inst.logicalID,
				Tags:      inst.tags,
			})
		}
	}

	return desc, nil
}

const (
	typeMinion = "minion"
	typeLeader = "leader"
)

type testFlavor struct {
	tags map[string]string
}

type schema struct {
	Type   string
	Size   uint
	Shards []instance.LogicalID
}

func (t testFlavor) Validate(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {

	s := schema{}
	err := json.Unmarshal(flavorProperties, &s)
	if err != nil {
		return flavor.AllocationMethod{}, nil
	}

	switch s.Type {
	case typeMinion:
		return flavor.AllocationMethod{Size: s.Size}, nil
	case typeLeader:
		return flavor.AllocationMethod{LogicalIDs: s.Shards}, nil
	default:
		return flavor.AllocationMethod{}, errors.New("Unrecognized node type")
	}
}

func (t testFlavor) Prepare(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	spec.Init = "echo hello"
	for k, v := range t.tags {
		spec.Tags[k] = v
	}
	return spec, nil
}

func (t testFlavor) Healthy(inst instance.Description) (bool, error) {
	return true, nil
}
