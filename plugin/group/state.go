package group

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/controller/util"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

// Supervisor watches over a group of instances.
type Supervisor interface {
	util.RunStop

	PlanUpdate(scaled Scaled, settings groupSettings, newSettings groupSettings) (updatePlan, error)
}

type configSchema struct {
	Size                     uint32
	IPs                      []string
	InstancePlugin           string
	InstancePluginProperties json.RawMessage
}

func instanceHash(config json.RawMessage) string {
	// First unmarshal and marshal the JSON to ensure stable key ordering.  This allows structurally-identical
	// JSON to yield the same hash even if the fields are reordered.

	props := map[string]interface{}{}
	err := json.Unmarshal(config, &props)
	if err != nil {
		panic(err)
	}

	stable, err := json.Marshal(props)
	if err != nil {
		panic(err)
	}

	hasher := sha1.New()
	hasher.Write(stable)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (c configSchema) instanceHash() string {
	return instanceHash(c.InstancePluginProperties)
}

type groupSettings struct {
	role   string
	plugin instance.Plugin
	config configSchema
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

func (c *groupContext) getUpdate() updatePlan {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.update
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
