package types

import (
	"fmt"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	defaultNamePrefix  = "instance"
	defaultMachineType = "g1-small"
	defaultNetwork     = "default"
	defaultDiskSizeMb  = 10
	defaultDiskImage   = "docker"
	defaultDiskType    = "pd-standard"
)

// Properties is the configuration schema for the plugin, provided in instance.Spec.Properties
type Properties struct {
	NamePrefix  string
	Description string
	MachineType string
	Network     string
	DiskSizeMb  int64
	DiskImage   string
	DiskType    string
	Tags        []string
	Scopes      []string
	TargetPools []string
	Connect     bool
	Preemptible bool
}

// ParseProperties parses instance Properties from a json description.
func ParseProperties(req *types.Any) (Properties, error) {
	parsed := Properties{
		NamePrefix:  defaultNamePrefix,
		MachineType: defaultMachineType,
		Network:     defaultNetwork,
		DiskSizeMb:  defaultDiskSizeMb,
		DiskImage:   defaultDiskImage,
		DiskType:    defaultDiskType,
	}

	if err := req.Decode(&parsed); err != nil {
		return parsed, fmt.Errorf("Invalid properties: %s", err)
	}

	return parsed, nil
}

// ParseMetadata returns a metadata key/value map from the instance specification.
func ParseMetadata(spec instance.Spec) (map[string]string, error) {
	metadata := make(map[string]string)
	for k, v := range spec.Tags {
		metadata[k] = v
	}

	if spec.Init != "" {
		metadata["startup-script"] = spec.Init
	}

	properties, err := ParseProperties(spec.Properties)
	if err != nil {
		return nil, err
	}
	if properties.Connect {
		metadata["serial-port-enable"] = "true"
	}

	return metadata, nil
}
