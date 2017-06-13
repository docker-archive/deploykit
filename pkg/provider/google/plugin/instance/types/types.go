package types

import (
	"fmt"

	"github.com/docker/infrakit/pkg/provider/google/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	defaultNamePrefix        = "instance"
	defaultDescription       = ""
	defaultMachineType       = "g1-small"
	defaultNetwork           = "default"
	defaultPreemptible       = false
	defaultDiskBoot          = true
	defaultDiskSizeGb        = int64(10)
	defaultDiskImage         = "docker"
	defaultDiskType          = "pd-standard"
	defaultDiskAutoDelete    = true
	defaultDiskReuseExisting = false

	// InfrakitLogicalID is a metadata key that is used to tag instances created with a LogicalId.
	InfrakitLogicalID = "infrakit-logical-id"

	// InfrakitGCPVersion is a metadata key that is used to know which version of the plugin was used to create
	// the instance.
	InfrakitGCPVersion = "infrakit-gcp-version"

	// InfrakitGCPCurrentVersion is incremented each time the plugin introduces incompatibilities with previous
	// versions
	InfrakitGCPCurrentVersion = "1"
)

// Properties is the configuration schema for the plugin, provided in instance.Spec.Properties
type Properties struct {
	*gcloud.InstanceSettings

	NamePrefix  string
	TargetPools []string
	Connect     bool
}

// ParseProperties parses instance Properties from a json description.
func ParseProperties(req *types.Any) (Properties, error) {
	parsed := Properties{
		NamePrefix: defaultNamePrefix,
		InstanceSettings: &gcloud.InstanceSettings{
			Description: defaultDescription,
			MachineType: defaultMachineType,
			Network:     defaultNetwork,
			Preemptible: defaultPreemptible,
			Disks: []gcloud.DiskSettings{
				{
					Boot:          defaultDiskBoot,
					SizeGb:        defaultDiskSizeGb,
					Image:         defaultDiskImage,
					Type:          defaultDiskType,
					AutoDelete:    defaultDiskAutoDelete,
					ReuseExisting: defaultDiskReuseExisting,
				},
			},
		},
	}

	if err := req.Decode(&parsed); err != nil {
		return parsed, fmt.Errorf("Invalid properties: %s", err)
	}

	return parsed, nil
}

// ParseTags returns a key/value map from the instance specification.
func ParseTags(spec instance.Spec) (map[string]string, error) {
	tags := make(map[string]string)

	for k, v := range spec.Tags {
		tags[k] = v
	}

	if spec.Init != "" {
		// spec.Init is special. Some plugins customise it via
		// the templating mechanism and it can either be a
		// startup script or just userdata. Store it twice.
		tags["startup-script"] = spec.Init
		tags["userdata"] = spec.Init
	}

	properties, err := ParseProperties(spec.Properties)
	if err != nil {
		return nil, err
	}
	if properties.Connect {
		tags["serial-port-enable"] = "true"
	}

	if spec.LogicalID != nil {
		tags[InfrakitLogicalID] = string(*spec.LogicalID)
	}

	tags[InfrakitGCPVersion] = InfrakitGCPCurrentVersion

	return tags, nil
}
