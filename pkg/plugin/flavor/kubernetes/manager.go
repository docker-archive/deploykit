package kubernetes

import (
	"errors"

	"github.com/docker/infrakit/pkg/discovery"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// NewManagerFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a kubernetes.
func NewManagerFlavor(plugins func() discovery.Plugins,
	templ *template.Template,
	dir string,
	stop <-chan struct{}) *ManagerFlavor {

	base := &baseFlavor{initScript: templ, plugins: plugins, kubeConfDir: dir}
	//	base.metadataPlugin = metadata.NewPluginFromChannel(base.runMetadataSnapshot(stop))
	return &ManagerFlavor{baseFlavor: base}
}

// ManagerFlavor is the flavor for kube managers
type ManagerFlavor struct {
	*baseFlavor
}

// Validate checks whether the helper can support a configuration.
func (s *ManagerFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {

	if err := s.baseFlavor.Validate(flavorProperties, allocation); err != nil {
		return err
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}

	if len(allocation.LogicalIDs) != 1 {
		return errors.New("kubernetes flaver currently support only one manager")
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
	instanceSpec instance.Spec, allocation group_types.AllocationMethod,
	index group_types.Index) (instance.Spec, error) {
	return s.baseFlavor.prepare("manager", flavorProperties, instanceSpec, allocation, index)
}
