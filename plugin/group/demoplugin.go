package scaler

import (
	"errors"
	"fmt"
	"github.com/docker/libmachete/spi/instance"
	"math/rand"
	"sync"
)

type demoplugin struct {
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

func newDemoPlugin() instance.Plugin {
	return &demoplugin{idPrefix: randString(4), instances: map[instance.ID]map[string]string{}}
}

func (d *demoplugin) Provision(req string, volume *instance.VolumeID, tags map[string]string) (*instance.ID, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.nextID++

	id := instance.ID(fmt.Sprintf("%s-%d", d.idPrefix, d.nextID))
	d.instances[id] = tags
	return &id, nil
}

func (d *demoplugin) Destroy(id instance.ID) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	_, exists := d.instances[id]
	if !exists {
		return errors.New("Instance does not exist")
	}

	delete(d.instances, id)
	return nil
}

func (d *demoplugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
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
