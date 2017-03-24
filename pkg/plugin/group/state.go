package group

import (
	"fmt"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/plugin/group/util"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"sync"
)

// Supervisor watches over a group of instances.
type Supervisor interface {
	util.RunStop

	ID() group.ID

	Size() uint

	PlanUpdate(scaled Scaled, settings groupSettings, newSettings groupSettings) (updatePlan, error)
}

type groupSettings struct {
	instancePlugin instance.Plugin
	flavorPlugin   flavor.Plugin
	config         types.Spec
}

type groupContext struct {
	settings   groupSettings
	supervisor Supervisor
	scaled     *scaledGroup
	update     updatePlan
	lock       sync.Mutex
}

func (c *groupContext) setUpdate(plan updatePlan) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.update = plan
}

func (c *groupContext) updating() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.update != nil
}

func (c *groupContext) stopUpdating() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.update != nil {
		c.update.Stop()
		c.update = nil
	}
}

func (c *groupContext) changeSettings(settings groupSettings) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.settings = settings
	c.scaled.changeSettings(settings)
}

type groups struct {
	byID map[group.ID]*groupContext
	lock sync.Mutex
}

func (g *groups) del(id group.ID) {
	g.lock.Lock()
	defer g.lock.Unlock()

	delete(g.byID, id)
}

func (g *groups) get(id group.ID) (*groupContext, bool) {
	g.lock.Lock()
	defer g.lock.Unlock()

	logical, exists := g.byID[id]
	return logical, exists
}

func (g *groups) put(id group.ID, context *groupContext) {
	g.lock.Lock()
	defer g.lock.Unlock()

	_, exists := g.byID[id]
	if exists {
		panic(fmt.Sprintf("Attempt to overwrite group %v", id))
	}

	g.byID[id] = context
}

func (g *groups) forEach(fn func(group.ID, *groupContext) error) error {
	g.lock.Lock()
	defer g.lock.Unlock()
	for id, ctx := range g.byID {
		if err := fn(id, ctx); err != nil {
			return err
		}
	}
	return nil
}

type sortByID []instance.Description

func (n sortByID) Len() int {
	return len(n)
}

func (n sortByID) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n sortByID) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}
