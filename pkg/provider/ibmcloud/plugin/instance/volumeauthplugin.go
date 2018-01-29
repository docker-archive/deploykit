package instance

import (
	"strconv"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/provider/ibmcloud/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// For logging
var (
	logger = logutil.New("module", "ibmcloud/volumeauth")

	debugV = logutil.V(500)
)

type plugin struct {
	SoftlayerClient client.API
	VolumeID        int
}

type propertyID struct {
	// Resource matches the resource structure of the tf.json resource section
	ID string `json:"id"`
}

// NewVolumeAuthPlugin creates a new plugin that manages VM authorizations to the given volume
func NewVolumeAuthPlugin(username, apiKey string, volumeID int) instance.Plugin {
	logger.Info("NewVolumeAuthPlugin", "volumeID", volumeID)
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
	logger.Info("Authorizing instance", "volume", p.VolumeID, "instance", props.ID)
	vmID, err := strconv.Atoi(props.ID)
	if err != nil {
		return nil, err
	}
	err = p.SoftlayerClient.AuthorizeToStorage(p.VolumeID, vmID)
	return nil, err
}

func (p *plugin) Destroy(id instance.ID, ctx instance.Context) error {
	logger.Info("Deauthorizing instance", "volume", p.VolumeID, "instance", string(id))
	vmID, err := strconv.Atoi(string(id))
	if err != nil {
		return err
	}
	return p.SoftlayerClient.DeauthorizeFromStorage(p.VolumeID, vmID)
}

func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	logger.Debug("Describing authorized VMs", "volume", p.VolumeID, "tags", tags, "V", debugV)
	vmIDs, err := p.SoftlayerClient.GetAllowedStorageVirtualGuests(p.VolumeID)
	if err != nil {
		return []instance.Description{}, err
	}
	result := []instance.Description{}
	for _, vmID := range vmIDs {
		result = append(result,
			instance.Description{
				ID: instance.ID(strconv.Itoa(vmID)),
			},
		)
	}
	logger.Debug("Authorized VMs", "volume", p.VolumeID, "VM-count", len(result), "VMs", result, "V", debugV)
	return result, nil
}
