package group

import (
	"errors"
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/controller/group/util"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// newTestInstancePlugin creates a new instance plugin for use in testing and development.
func newTestInstancePlugin(seedInstances ...instance.Spec) *testplugin {
	plugin := testplugin{idPrefix: util.RandomAlphaNumericString(4), instances: map[instance.ID]instance.Spec{}}
	for _, inst := range seedInstances {
		plugin.addInstance(inst)
	}

	return &plugin
}

type testplugin struct {
	lock      sync.Mutex
	idPrefix  string
	nextID    int
	instances map[instance.ID]instance.Spec

	destroyed []instance.Spec
}

func (d *testplugin) instancesCopy() map[instance.ID]instance.Spec {
	d.lock.Lock()
	defer d.lock.Unlock()

	instances := map[instance.ID]instance.Spec{}
	for k, v := range d.instances {
		instances[k] = v
	}
	return instances
}

func (d *testplugin) Validate(req *types.Any) error {
	return nil
}

func (d *testplugin) addInstance(spec instance.Spec) instance.ID {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.nextID++
	id := instance.ID(fmt.Sprintf("%s-%d", d.idPrefix, d.nextID))
	d.instances[id] = spec
	return id
}

func (d *testplugin) Provision(spec instance.Spec) (*instance.ID, error) {

	id := d.addInstance(spec)
	return &id, nil
}

func (d *testplugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (d *testplugin) Destroy(id instance.ID, ctx instance.Context) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	spec, exists := d.instances[id]
	if !exists {
		return errors.New("Instance does not exist")
	}
	if _, has := spec.Tags["DestroyError"]; has {
		return errors.New("DestroyError")
	}
	delete(d.instances, id)

	d.destroyed = append(d.destroyed, spec)
	return nil
}

func (d *testplugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	desc := []instance.Description{}
	for id, inst := range d.instances {
		allMatched := true
		for searchKey, searchValue := range tags {
			tagValue, has := inst.Tags[searchKey]
			if !has || searchValue != tagValue {
				allMatched = false
			}
		}
		if allMatched {
			desc = append(desc, instance.Description{
				ID:        id,
				LogicalID: inst.LogicalID,
				Tags:      inst.Tags,
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
	healthy func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error)
	drain   func(flavorProperties *types.Any, inst instance.Description) error

	lock    sync.Mutex
	drained []instance.Description
}

type flavorSchema struct {
	Type string
	Init string
	Tags map[string]string
}

func (t *testFlavor) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {

	s := flavorSchema{}
	err := flavorProperties.Decode(&s)
	if err != nil {
		return err
	}

	switch s.Type {
	case typeMinion:
		if len(allocation.LogicalIDs) > 0 {
			return errors.New("Minion Groups must be scaled with Size, not LogicalIDs")
		}
		return nil
	case typeLeader:
		if allocation.Size > 0 {
			return errors.New("Leader Groups must be scaled with LogicalIDs, not Size")
		}
		return nil
	default:
		return errors.New("Unrecognized node type")
	}
}

func (t *testFlavor) Prepare(flavorProperties *types.Any,
	spec instance.Spec,
	allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {

	s := flavorSchema{}
	err := flavorProperties.Decode(&s)
	if err != nil {
		return spec, err
	}

	spec.Init = s.Init
	for k, v := range s.Tags {
		spec.Tags[k] = v
	}
	return spec, nil
}

func (t *testFlavor) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	if t.healthy != nil {
		return t.healthy(flavorProperties, inst)
	}

	return flavor.Healthy, nil
}

func (t *testFlavor) Drain(flavorProperties *types.Any, inst instance.Description) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.drained = append(t.drained, inst)
	if t.drain != nil {
		return t.drain(flavorProperties, inst)
	}

	return nil
}
