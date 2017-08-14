package selector

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Options container configuration parameter of the plugin
type Options []Choice

// Choice represents one of the possible instance plugins
type Choice struct {
	Name      plugin.Name
	Instances []instance.LogicalID
	Affinity  *types.Any
}

// HasLogicalID returns true if the choice contains a rule binding the given instance id.
func (c Choice) HasLogicalID(id instance.LogicalID) bool {
	for _, logicalID := range c.Instances {
		if logicalID == id {
			return true
		}
	}
	return false
}

func (o Options) Len() int           { return len(o) }
func (o Options) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o Options) Less(i, j int) bool { return o[i].Name < o[j].Name }
