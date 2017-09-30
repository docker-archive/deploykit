package swarm

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/infrakit/pkg/discovery"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

// NewWorkerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewWorkerFlavor(plugins func() discovery.Plugins, connect func(Spec) (docker.APIClientCloser, error),
	templ *template.Template,
	stop <-chan struct{}) *WorkerFlavor {

	base := &baseFlavor{initScript: templ, getDockerClient: connect, plugins: plugins}
	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &WorkerFlavor{baseFlavor: base}
}

// WorkerFlavor implements the flavor and metadata plugins
type WorkerFlavor struct {
	*baseFlavor
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the swarm to join.
func (s *WorkerFlavor) Prepare(flavorProperties *types.Any, instanceSpec instance.Spec,
	allocation group_types.AllocationMethod,
	index group_types.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("worker", flavorProperties, instanceSpec, allocation, index)
}

// Drain in the case of worker will force a node removal in the swarm.
func (s *WorkerFlavor) Drain(flavorProperties *types.Any, inst instance.Description) error {
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

	dockerClient, err := s.baseFlavor.getDockerClient(spec)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	nodes, err := dockerClient.NodeList(context.Background(), docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return err
	}

	switch {
	case len(nodes) == 0:
		return fmt.Errorf("Unable to drain %s, not found in swarm", inst.ID)

	case len(nodes) == 1:
		log.Debugln("Docker NodeRemove", nodes[0].ID)
		err := dockerClient.NodeRemove(
			context.Background(),
			nodes[0].ID,
			docker_types.NodeRemoveOptions{Force: true})
		if err != nil {
			return err
		}

		return nil

	default:
		return fmt.Errorf("Expected at most one node with label %s, but found %v", link.Value(), nodes)
	}
}
