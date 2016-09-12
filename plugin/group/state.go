package scaler

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

type groupProperties struct {
	Size                     uint32
	InstancePlugin           string
	InstancePluginProperties json.RawMessage
}

type groupContext struct {
	properties     *groupProperties
	instancePlugin instance.Plugin
	scaler         Scaler
	scaled         *scaledGroup
	update         updatePlan
	lock           sync.Mutex
}

func (c *groupContext) setUpdate(plan updatePlan) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.update = plan
}

func (c *groupContext) getUpdate() updatePlan {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.update
}

func (c *groupContext) setProperties(properties *groupProperties) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.properties = properties
}

type groups struct {
	contexts map[group.ID]*groupContext
	lock     sync.Mutex
}

func (g *groups) del(id group.ID) {
	g.lock.Lock()
	defer g.lock.Unlock()

	delete(g.contexts, id)
}

func (g *groups) get(id group.ID) (*groupContext, bool) {
	g.lock.Lock()
	defer g.lock.Unlock()

	logical, exists := g.contexts[id]
	return logical, exists
}

func (g *groups) put(id group.ID, context *groupContext) {
	g.lock.Lock()
	defer g.lock.Unlock()

	_, exists := g.contexts[id]
	if exists {
		panic(fmt.Sprintf("Attempt to overwrite group %v", id))
	}

	g.contexts[id] = context
}

type sortByID []instance.ID

func (n sortByID) Len() int {
	return len(n)
}

func (n sortByID) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n sortByID) Less(i, j int) bool {
	return n[i] < n[j]
}
