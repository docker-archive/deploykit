package swarm

import (
	"errors"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
)

const (
	roleWorker  = "worker"
	roleManager = "manager"
)

// NewSwarmProvisionHelper creates a ProvisionHelper that creates manager and worker nodes connected in a swarm.
func NewSwarmProvisionHelper() types.ProvisionHelper {
	return &swarmProvisioner{}
}

type swarmProvisioner struct {
}

// TODO(wfarner): Tag with with the Swarm cluster UUID for scoping.

// TODO(wfarner): Tag instances with a UUID, and tag the Docker engine with the same UUID.  We will use this to
// associate swarm nodes with instances.

// TODO(wfarner): Add a ProvisionHelper function to check the health of an instance.  Use the Swarm node association
// (see TODO above) to provide this.

func (s swarmProvisioner) Validate(config group.Configuration, parsed types.Schema) error {
	if config.Role == roleManager {
		if len(parsed.IPs) != 1 && len(parsed.IPs) != 3 && len(parsed.IPs) != 5 {
			return errors.New("Must have 1, 3, or 5 managers")
		}
	}
	return nil
}

func (s swarmProvisioner) GroupKind(roleName string) (types.GroupKind, error) {
	switch roleName {
	case roleWorker:
		return types.KindDynamicIP, nil
	case roleManager:
		return types.KindStaticIP, nil
	default:
		return types.KindNone, errors.New("Unsupported role type")
	}
}

func (s swarmProvisioner) PreProvision(
	config group.Configuration,
	details types.ProvisionDetails) (types.ProvisionDetails, error) {

	// TODO(wfarner): Generate user data based on the machine role.
	switch config.Role {
	case roleWorker:
	case roleManager:
		if details.PrivateIP == nil {
			return details, errors.New("Manager nodes require an assigned private IP address")
		}

		volume := instance.VolumeID(*details.PrivateIP)
		details.Volume = &volume

	default:
		return details, errors.New("Unsupported role type")
	}

	return details, nil
}
