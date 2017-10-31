package kubernetes

import (
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a kubernetes.
func NewManagerFlavor(scope scope.Scope, options Options, stop <-chan struct{}) (*ManagerFlavor, error) {

	mt, err := getTemplate(options.DefaultManagerInitScriptTemplate,
		DefaultManagerInitScriptTemplate, DefaultTemplateOptions)

	if err != nil {
		return nil, err
	}

	base := &baseFlavor{
		initScript: mt,
		scope:      scope,
		options:    options,
	}
	//	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &ManagerFlavor{baseFlavor: base}, nil
}

// ManagerFlavor is the flavor for kube managers
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

	for _, id := range allocation.LogicalIDs {
		if att, exists := spec.Attachments[id]; !exists || len(att) == 0 {
			log.Warn("Instance has no attachments, which is needed for durability", "logicalID", id)
		}
	}
	ads := map[string]string{}
	for _, a := range spec.KubeAddOns {
		if a.Type == "network" || a.Type == "visualise" {
			ads[a.Type] = a.Name
		}
	}
	if _, ok := ads["network"]; !ok {
		log.Warn("No Network addon configured. Your cluster will not be Ready status until apply network addon.")
	}
	for k, v := range ads {
		log.Info("Apply addon", "type", k, "name", v)
	}
	return nil
}

// Prepare sets up the provisioner / instance plugin's spec based on information about the kubernetes to join.
func (s *ManagerFlavor) Prepare(flavorProperties *types.Any,
	instanceSpec instance.Spec, allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("manager", flavorProperties, instanceSpec, allocation, index)
}
