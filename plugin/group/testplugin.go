package group

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/plugin/group/util"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

type fakeInstance struct {
	ip   *string
	tags map[string]string
}

// NewTestInstancePlugin creates a new instance plugin for use in testing and development.
func NewTestInstancePlugin(seedInstances ...map[string]string) instance.Plugin {
	plugin := testplugin{idPrefix: util.RandomAlphaNumericString(4), instances: map[instance.ID]fakeInstance{}}
	for _, i := range seedInstances {
		plugin.addInstance(fakeInstance{tags: i})
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

	id := d.addInstance(fakeInstance{ip: spec.PrivateIPAddress, tags: spec.Tags})
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
			ip := ""
			if inst.ip != nil {
				ip = *inst.ip
			}

			desc = append(desc, instance.Description{
				ID:               id,
				PrivateIPAddress: ip,
				Tags:             inst.tags,
			})
		}
	}

	return desc, nil
}

const (
	roleMinions = "minions"
	roleLeaders = "leaders"
)

type testProvisionHelper struct {
	tags map[string]string
}

func (t testProvisionHelper) Validate(config group.Configuration, parsed types.Schema) error {
	return nil
}

func (t testProvisionHelper) GroupKind(roleName string) types.GroupKind {
	switch roleName {
	case roleMinions:
		return types.KindDynamicIP
	case roleLeaders:
		return types.KindStaticIP
	default:
		return types.KindUnknown
	}
}

func (t testProvisionHelper) PreProvision(
	config group.Configuration, spec instance.Spec) (instance.Spec, error) {

	spec.InitScript = "echo hello"
	for k, v := range t.tags {
		spec.Tags[k] = v
	}
	return spec, nil
}

func (t testProvisionHelper) Healthy(inst instance.Description) (bool, error) {
	return true, nil
}
