package instance

import (
	"github.com/codedellemc/gorackhd/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type rackHDInstancePlugin struct {
	client client.Monorail
}

// NewInstancePlugin creates a new plugin that creates instances in RackHD.
func NewInstancePlugin(client *client.Monorail) instance.Plugin {
	//return &rackHDInstancePlugin{client: client}
	return nil
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

func (p rackHDInstancePlugin) Provision(pec instance.Spec) (*instance.ID, error) {
	return nil, nil
}

func (p rackHDInstancePlugin) Validate(req *types.Any) error {
	return nil
}
