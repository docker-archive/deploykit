package kubernetes

import (
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// NewWorkerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a kubernetes.
func NewWorkerFlavor(scope scope.Scope, options Options, stop <-chan struct{}) (*WorkerFlavor, error) {

	wt, err := getTemplate(options.DefaultWorkerInitScriptTemplate,
		DefaultWorkerInitScriptTemplate, DefaultTemplateOptions)

	if err != nil {
		return nil, err
	}

	base := &baseFlavor{initScript: wt, scope: scope, options: options}
	//	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &WorkerFlavor{baseFlavor: base}, nil
}

// WorkerFlavor is the flavor for kubernetes workers
type WorkerFlavor struct {
	*baseFlavor
}

// Validate checks whether the helper can support a configuration.
func (s *WorkerFlavor) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {

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
	instanceSpec instance.Spec, allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("worker", flavorProperties, instanceSpec, allocation, index)
}
