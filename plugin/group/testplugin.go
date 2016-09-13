package scaler

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/libmachete/spi/instance"
	"math/rand"
	"sync"
)

// NewTestInstancePlugin creates a new instance plugin for use in testing and development.
func NewTestInstancePlugin(seedInstances ...map[string]string) instance.Plugin {
	plugin := testplugin{idPrefix: randString(4), instances: map[instance.ID]map[string]string{}}
	for _, i := range seedInstances {
		plugin.addInstance(i)
	}

	return &plugin
}

type testplugin struct {
	lock      sync.Mutex
	idPrefix  string
	nextID    int
	instances map[instance.ID]map[string]string
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (d *testplugin) Validate(req json.RawMessage) error {
	return nil
}

func (d *testplugin) addInstance(tags map[string]string) instance.ID {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.nextID++
	id := instance.ID(fmt.Sprintf("%s-%d", d.idPrefix, d.nextID))
	d.instances[id] = tags
	return id
}

func (d *testplugin) Provision(
	req json.RawMessage,
	volume *instance.VolumeID,
	tags map[string]string) (*instance.ID, error) {

	id := d.addInstance(tags)
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
	for id, instanceTags := range d.instances {
		allMatched := true
		for searchKey, searchValue := range tags {
			tagValue, has := instanceTags[searchKey]
			if !has || searchValue != tagValue {
				allMatched = false
			}
		}
		if allMatched {
			desc = append(desc, instance.Description{
				ID:               id,
				PrivateIPAddress: "none",
				Tags:             instanceTags,
			})
		}
	}

	return desc, nil
}
