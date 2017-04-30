package types

import (
	"github.com/digitalocean/godo"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/pkg/errors"
)

const (
	// InfrakitLogicalID is a metadata key that is used to tag instances created with a LogicalId.
	InfrakitLogicalID = "infrakit-logical-id"

	// InfrakitDOVersion is a metadata key that is used to know which version of the plugin was used to create
	// the instance.
	InfrakitDOVersion = "infrakit-do-version"

	// InfrakitDOCurrentVersion is incremented each time the plugin introduces incompatibilities with previous
	// versions
	InfrakitDOCurrentVersion = "1"
)

// Properties is the configuration schema for the plugin, provided in instance.Spec.Properties
type Properties struct {
	godo.DropletCreateRequest

	NamePrefix  string
	SSHKeyNames []string
	// Image             string
	// Size              string
	// Backups           bool
	// IPv6              bool `json:"ipv6"`
	// PrivateNetworking bool `json:"private_networking"`
	// Tags              []string
}

// ParseProperties parses instance Properties from a json description.
func ParseProperties(req *types.Any) (Properties, error) {
	// FIXME(vdemeester) add default values (see infrakit.gcp)
	parsed := Properties{}

	if err := req.Decode(&parsed); err != nil {
		return parsed, errors.Wrap(err, "invalid properties")
	}
	return parsed, nil
}

// ParseTags returns a key/value map from the instance specification.
func ParseTags(spec instance.Spec) map[string]string {
	tags := make(map[string]string)

	for k, v := range spec.Tags {
		tags[k] = v
	}

	// Do stuff with proprerties here

	if spec.LogicalID != nil {
		tags[InfrakitLogicalID] = string(*spec.LogicalID)
	}

	tags[InfrakitDOVersion] = InfrakitDOCurrentVersion

	return tags
}
