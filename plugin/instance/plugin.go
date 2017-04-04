package instance

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	instance_types "github.com/docker/infrakit.gcp/plugin/instance/types"
	"github.com/docker/infrakit.gcp/plugin/instance/util"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type plugin struct {
	API gcloud.API
}

// NewGCEInstancePlugin creates a new GCE instance plugin for a given project
// and zone.
func NewGCEInstancePlugin(project, zone string) instance.Plugin {
	api, err := gcloud.New(project, zone)
	if err != nil {
		log.Fatal(err)
	}

	return &plugin{
		API: api,
	}
}

func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-gcp",
			Version: "0.3.0",
		},
		URL: "https://github.com/docker/infrakit.gcp",
	}
}

func (p *plugin) Validate(req *types.Any) error {
	log.Debugln("validate", req.String())

	parsed := instance_types.Properties{}
	return req.Decode(&parsed)
}

func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	metadata := gcloud.TagsToMetaData(labels)

	return p.API.AddInstanceMetadata(string(instance), metadata)
}

func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	properties, err := instance_types.ParseProperties(spec.Properties)
	if err != nil {
		return nil, err
	}

	metadata, err := instance_types.ParseMetadata(spec)
	if err != nil {
		return nil, err
	}

	var name string
	if spec.LogicalID != nil {
		name = string(*spec.LogicalID)
	} else {
		name = fmt.Sprintf("%s-%s", properties.NamePrefix, util.RandomSuffix(6))
	}
	id := instance.ID(name)

	if err = p.API.CreateInstance(name, &gcloud.InstanceSettings{
		Description:       properties.Description,
		MachineType:       properties.MachineType,
		Network:           properties.Network,
		Tags:              properties.Tags,
		DiskSizeMb:        properties.DiskSizeMb,
		DiskImage:         properties.DiskImage,
		DiskType:          properties.DiskType,
		Scopes:            properties.Scopes,
		Preemptible:       properties.Preemptible,
		AutoDeleteDisk:    spec.LogicalID == nil,
		ReuseExistingDisk: spec.LogicalID != nil,
		MetaData:          gcloud.TagsToMetaData(metadata),
	}); err != nil {
		return nil, err
	}

	for _, targetPool := range properties.TargetPools {
		if err = p.API.AddInstanceToTargetPool(targetPool, name); err != nil {
			return nil, err
		}
	}

	return &id, nil
}

func (p *plugin) Destroy(id instance.ID) error {
	err := p.API.DeleteInstance(string(id))

	log.Debugln("destroy", id, "err=", err)

	return err
}

func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)

	instances, err := p.API.ListInstances()
	if err != nil {
		return nil, err
	}

	log.Debugln("total count:", len(instances))

	result := []instance.Description{}

	for _, inst := range instances {
		instTags := gcloud.MetaDataToTags(inst.Metadata.Items)
		if gcloud.HasDifferentTag(tags, instTags) {
			continue
		}

		description := instance.Description{
			ID:   instance.ID(inst.Name),
			Tags: instTags,
		}

		// When pets are deleted, we keep the disk. So a machine with a disk that's not auto-deleted is
		// assumed to be a pet and its logicalID is the name of the disk.
		if len(inst.Disks) > 0 && !inst.Disks[0].AutoDelete {
			id := instance.LogicalID(last(inst.Disks[0].Source))
			description.LogicalID = &id
		}

		if properties {
			if any, err := types.AnyValue(inst); err == nil {
				description.Properties = any
			} else {
				log.Warningln("error encoding instance properties:", err)
			}
		}
		result = append(result, description)
	}

	log.Debugln("matching count:", len(result))

	return result, nil
}

func last(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
