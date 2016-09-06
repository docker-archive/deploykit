package scaler

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/spi/group"
)

type groupProperties struct {
	Size                     uint32
	InstancePlugin           string
	InstancePluginProperties json.RawMessage
}

type physicalGroup struct {
	properties *groupProperties
	scaler     Scaler
}

func (p *physicalGroup) setSize(size uint32) {
	p.properties.Size = size
	p.scaler.SetSize(size)
}

type physicalGroupID struct {
	gid   group.ID
	phyID int
}

type logicalGroup struct {
	phys map[int]*physicalGroup
}

func (l *logicalGroup) getPhy(id int) (*physicalGroup, bool) {
	phy, exists := l.phys[id]
	return phy, exists
}

func (l *logicalGroup) deletePhy(id int) {
	delete(l.phys, id)
}

func (l *logicalGroup) getOnlyPhy() (int, *physicalGroup, error) {
	if len(l.phys) == 1 {
		for id, gen := range l.phys {
			return id, gen, nil
		}
	}

	return -1, nil, fmt.Errorf("Logical group has %d physical groups, expected 1", len(l.phys))
}

type groups struct {
	logical map[group.ID]*logicalGroup
}

func (g *groups) deleteLogical(id group.ID) {
	delete(g.logical, id)
}

func (g *groups) get(id group.ID) (*logicalGroup, bool) {
	logical, exists := g.logical[id]
	return logical, exists
}

func (g *groups) putPhy(id physicalGroupID, phy *physicalGroup) {
	logical, exists := g.logical[id.gid]
	if !exists {
		logical = &logicalGroup{phys: map[int]*physicalGroup{}}
		g.logical[id.gid] = logical
	}

	_, exists = logical.phys[id.phyID]
	if exists {
		panic(fmt.Sprintf("Attempt to overwrite physical group %v", id))
	}

	logical.phys[id.phyID] = phy
}

func (g *groups) getPhy(id physicalGroupID) (*physicalGroup, bool) {
	logical, exists := g.logical[id.gid]
	if !exists {
		return nil, false
	}

	phy, exists := logical.phys[id.phyID]
	return phy, exists
}
