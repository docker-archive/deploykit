package swarm

import (
	"errors"
	"fmt"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewManagerFlavor(scope scope.Scope, connect func(Spec) (docker.APIClientCloser, error),
	templ *template.Template,
	stop <-chan struct{}, self *instance.LogicalID) *ManagerFlavor {

	base := &baseFlavor{initScript: templ, getDockerClient: connect, scope: scope, self: self}
	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &ManagerFlavor{baseFlavor: base}
}

// ManagerFlavor is the flavor for swarm managers
type ManagerFlavor struct {
	*baseFlavor
}

// Validate checks whether the helper can support a configuration.
func (s *ManagerFlavor) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {

	if err := s.baseFlavor.Validate(flavorProperties, allocation); err != nil {
		return err
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}

	if len(allocation.LogicalIDs)%2 == 0 {
		return errors.New("must have odd number for quorum")
	}

	for _, id := range allocation.LogicalIDs {
		if att, exists := spec.Attachments[id]; !exists || len(att) == 0 {
			log.Warn("No attachments, which is needed for durability", "id", id)
		}
	}
	return nil
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *ManagerFlavor) Prepare(flavorProperties *types.Any,
	instanceSpec instance.Spec, allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("manager", flavorProperties, instanceSpec, allocation, index)
}

// Drain in the case of manager, first perform a swarm node demote to
// downgrade the manager to a worker, then do a swarm leave.
// Note that if the current node is the leader running this code, the demote
// will turn the manager to a worker, and it's not possible to issue a
// docker node rm anymore because this node is no longer a manager and only
// manager nodes permit `docker node rm`.  So the node demote will be followed
// by `docker swarm leave` of *this* node.  This in essence takes the current
// leader node out of the swarm.
func (s *ManagerFlavor) Drain(flavorProperties *types.Any, inst instance.Description) error {
	if flavorProperties == nil {
		return fmt.Errorf("missing config")
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}

	link := types.NewLinkFromMap(inst.Tags)
	if !link.Valid() {
		return fmt.Errorf("Unable to drain %s without an association tag", inst.ID)
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", link.Label(), link.Value()))

	dockerClient, err := s.getDockerClient(spec)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	ctx := context.Background()

	nodes, err := dockerClient.NodeList(ctx, docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return err
	}

	switch {
	case len(nodes) == 0:
		return fmt.Errorf("not found %v", inst.ID)

	case len(nodes) == 1:

		// Do a swarm leave if and only if this is a manager

		nodeID := nodes[0].ID

		// first read the swarm version which is needed to update node
		sw, err := dockerClient.SwarmInspect(ctx)
		if err != nil {
			return err
		}

		version := sw.ClusterInfo.Meta.Version

		// then read the state of the node
		nodeInfo, _, err := dockerClient.NodeInspectWithRaw(ctx, nodeID)
		if err != nil {
			return err
		}

		if nodeInfo.Spec.Role != swarm.NodeRoleManager {
			return fmt.Errorf("not a manager: %v", nodeID)
		}

		// change to worker
		nodeInfo.Spec.Role = swarm.NodeRoleWorker

		log.Debug("Docker NodeDemote", "id", nodeID)
		err = dockerClient.NodeUpdate(
			ctx,
			nodeID,
			version,
			nodeInfo.Spec)
		if err != nil {
			return err
		}

		// If running on the same node (self), then do docker swarm leave
		// otherwise, remove the node
		if s.isSelf(inst) {

			log.Debug("Docker SwarmLeave", "id", nodeID)

			err := dockerClient.SwarmLeave(ctx, true)
			if err != nil {
				return err
			}

		} else {
			log.Debug("Docker NodeRemote", "id", nodeID)

			err := dockerClient.NodeRemove(
				ctx,
				nodeID,
				docker_types.NodeRemoveOptions{Force: true})
			if err != nil {
				return err
			}
		}

		return nil

	default:
		return fmt.Errorf("Expected at most one node with label %s, but found %v", link.Value(), nodes)
	}
}
