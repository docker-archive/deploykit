package instance

import (
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/ibmcloud/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type plugin struct {
	SoftlayerClient *client.SoftlayerClient
	VolumeID        int
}

type propertyID struct {
	// Resource matches the resource structure of the tf.json resource section
	ID string `json:"id"`
}

// NewVolumeAuthPlugin creates a new plugin that manages VM authorizations to the given volume
func NewVolumeAuthPlugin(username, apiKey string, volumeID int) instance.Plugin {
	log.Infof("NewVolumeAuthPlugin, volumeID: %v", volumeID)
	return &plugin{
		SoftlayerClient: client.GetClient(username, apiKey),
		VolumeID:        volumeID,
	}
}

func (p *plugin) Validate(req *types.Any) error {
	return nil
}

func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	return nil
}

func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	props := propertyID{}
	err := spec.Properties.Decode(&props)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	log.Infof("Authorizing volume %v to instance %v", p.VolumeID, props.ID)
	vmID, err := strconv.Atoi(props.ID)
	if err != nil {
		return nil, err
	}
	err = p.SoftlayerClient.AuthorizeToStorage(p.VolumeID, vmID)
	return nil, err
}

func (p *plugin) Destroy(id instance.ID, ctx instance.Context) error {
	log.Infof("Deauthorizing volume %v from instance %v", p.VolumeID, string(id))
	vmID, err := strconv.Atoi(string(id))
	if err != nil {
		return nil
	}
	return p.SoftlayerClient.DeauthorizeFromStorage(p.VolumeID, vmID)
}

func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Infof("Describing authorized VMs for volume %v with tags %v", p.VolumeID, tags)
	vmIDs, err := p.SoftlayerClient.GetAllowedStorageVirtualGuests(p.VolumeID)
	if err != nil {
		return []instance.Description{}, nil
	}
	result := []instance.Description{}
	for _, vmID := range vmIDs {
		result = append(result,
			instance.Description{
				ID: instance.ID(strconv.Itoa(vmID)),
			},
		)
	}
	log.Infof("%v authorized VMs for volume %v: %v", len(result), p.VolumeID, result)
	return result, nil
}
