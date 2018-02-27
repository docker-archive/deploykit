package swarm

import (
	"fmt"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
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

// DescribeInstances returns a slice of instance.Description objects, each having:
// - Docker node ID as ID
// - Docker engine labels as Tags
// - Docker node "infrakit-link" engine label as the LogicalID (if set)
// - Docker node data as Properties
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

// Destroy removes the node with the given instance ID from the swarm. If the node is currently
// a manager then it is demoted prior to removal.
func (s *InstancePlugin) Destroy(instance instance.ID, instContext instance.Context) error {
	dockerClient, err := s.base.getDockerClient(Spec{Docker: s.connectinfo})
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Retrieve the swarm node with the given ID
	ctx := context.Background()
	nodeID := string(instance)
	nodeInfo, _, err := dockerClient.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		if client.IsErrNotFound(err) {
			log.Warn("Unable to remove from swarm - not found in swarm", "id", nodeID)
			return nil
		}
		log.Info("Swarm node removal, failed to inspect node",
			"id", nodeID,
			"error", err)
		return err
	}

	// If the node is a manager then demote
	if nodeInfo.Spec.Role == swarm.NodeRoleManager {
		nodeInfo.Spec.Role = swarm.NodeRoleWorker
		if err := dockerClient.NodeUpdate(ctx, nodeID, nodeInfo.Version, nodeInfo.Spec); err != nil {
			log.Warn("Swarm node removal, failed to demote manager",
				"hostname", nodeInfo.Description.Hostname,
				"id", nodeID,
				"error", err)
			return err
		}
		log.Info("Swarm node removal, successfully demoted manager",
			"hostname", nodeInfo.Description.Hostname,
			"id", nodeID)
	}

	// And remove
	if err := dockerClient.NodeRemove(ctx, nodeID, docker_types.NodeRemoveOptions{Force: true}); err != nil {
		log.Warn("Swarm node removal, failed to remove node",
			"hostname", nodeInfo.Description.Hostname,
			"id", nodeID,
			"error", err)
		return err
	}
	log.Info("Successfully removed node from swarm",
		"hostname", nodeInfo.Description.Hostname,
		"id", instance)
	return nil

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
