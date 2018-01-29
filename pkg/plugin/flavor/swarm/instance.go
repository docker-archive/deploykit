package swarm

import (
	"fmt"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

// NewInstancePlugin creates the swarm instance plugin
func NewInstancePlugin(connectFn func(Spec) (docker.APIClientCloser, error), connectInfo docker.ConnectInfo) instance.Plugin {
	base := &baseFlavor{
		getDockerClient: connectFn,
		scope:           nil,
	}
	return &InstancePlugin{
		base:        base,
		connectinfo: connectInfo,
	}
}

// InstancePlugin is the instance plugin
type InstancePlugin struct {
	base        *baseFlavor
	connectinfo docker.ConnectInfo
}

// DescribeInstances .
func (s *InstancePlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	dockerClient, err := s.base.getDockerClient(Spec{Docker: s.connectinfo})
	if err != nil {
		return []instance.Description{}, err
	}
	defer dockerClient.Close()
	// Retrieve all swarm nodes
	nodes, err := dockerClient.NodeList(context.Background(), docker_types.NodeListOptions{})
	if err != nil {
		return []instance.Description{}, err
	}
	result := []instance.Description{}
	for _, n := range nodes {
		var propsAny *types.Any
		if properties {
			propsAny, err = types.AnyValue(n)
			if err != nil {
				log.Error("DescribeInstances", "msg", "Failed to encode node properties", "error", err)
				return []instance.Description{}, err
			}
		}
		// Use the engine labels as the tags, adding in the node name
		tags := n.Description.Engine.Labels
		if tags == nil {
			tags = map[string]string{}
		}
		tags["name"] = n.Description.Hostname
		// Use the LinkLabel engine label as the LogicalID
		var logicalID *instance.LogicalID
		if linkLabel, has := tags[types.LinkLabel]; has {
			v := instance.LogicalID(linkLabel)
			logicalID = &v
		}
		d := instance.Description{
			ID:         instance.ID(n.ID),
			LogicalID:  logicalID,
			Properties: propsAny,
			Tags:       tags,
		}
		result = append(result, d)
	}
	return result, nil
}

// Destroy .
func (s *InstancePlugin) Destroy(instance instance.ID, context instance.Context) error {
	dockerClient, err := s.base.getDockerClient(Spec{Docker: s.connectinfo})
	if err != nil {
		return err
	}
	defer dockerClient.Close()
	// TODO: remove node from swarm
	return fmt.Errorf("Destroy not yet supported for swarm instance")
}

// Validate is not suported
func (s *InstancePlugin) Validate(req *types.Any) error {
	return fmt.Errorf("Validate not supported for swarm instance")
}

// Provision is not suported
func (s *InstancePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	return nil, fmt.Errorf("Provision not supported for swarm instance")
}

// Label is not suported
func (s *InstancePlugin) Label(instance instance.ID, labels map[string]string) error {
	return fmt.Errorf("Label not supported for swarm instance")
}
