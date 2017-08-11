package kubernetes

import (
	"github.com/docker/infrakit/pkg/discovery"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// NewWorkerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a kubernetes.
func NewWorkerFlavor(plugins func() discovery.Plugins,
	templ *template.Template,
	dir string,
	stop <-chan struct{}) *WorkerFlavor {

	base := &baseFlavor{initScript: templ, plugins: plugins, kubeConfDir: dir}
	//	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &WorkerFlavor{baseFlavor: base}
}

// WorkerFlavor is the flavor for kubernetes workers
type WorkerFlavor struct {
	*baseFlavor
}

// Validate checks whether the helper can support a configuration.
func (s *WorkerFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {

	if err := s.baseFlavor.Validate(flavorProperties, allocation); err != nil {
		return err
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}
	return nil
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the kubernetes to join.
func (s *WorkerFlavor) Prepare(flavorProperties *types.Any,
	instanceSpec instance.Spec, allocation group_types.AllocationMethod,
	index group_types.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("worker", flavorProperties, instanceSpec, allocation, index)
}
