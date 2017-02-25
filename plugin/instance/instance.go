package instance

import (
	"github.com/codedellemc/infrakit.rackhd/monorail"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type rackHDInstancePlugin struct {
	client monorail.Iface
}

// NewInstancePlugin creates a new plugin that creates instances in RackHD.
func NewInstancePlugin(client monorail.Iface) instance.Plugin {
	return &rackHDInstancePlugin{client: client}
}

func (p rackHDInstancePlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return nil, nil
}

func (p rackHDInstancePlugin) Destroy(id instance.ID) error {
	return nil
}

func (p rackHDInstancePlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p rackHDInstancePlugin) Provision(_spec instance.Spec) (*instance.ID, error) {
	nodes, nil := p.client.Nodes().GetNodes(nil, nil)
	var nodeID instance.ID
	for _, node1 := range nodes.Payload {
		if string(node1.Type) == "compute" {
			nodeID = instance.ID(node1.ID)
			break
		}
	}
	return &nodeID, nil
}

func (p rackHDInstancePlugin) Validate(req *types.Any) error {
	return nil
}
