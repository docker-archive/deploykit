package instance

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	"github.com/docker/infrakit.gcp/plugin/instance/types"
	"github.com/docker/infrakit.gcp/plugin/instance/util"
	"github.com/docker/infrakit/pkg/spi/instance"
)

type plugin struct {
	API func() (gcloud.API, error)
}

// NewGCEInstancePlugin creates a new GCE instance plugin for a given project
// and zone.
func NewGCEInstancePlugin(project, zone string) instance.Plugin {
	_, err := gcloud.New(project, zone)
	if err != nil {
		log.Fatal(err)
	}

	return &plugin{
		API: func() (gcloud.API, error) {
			return gcloud.New(project, zone)
		},
	}
}

func (p *plugin) Validate(req json.RawMessage) error {
	log.Debugln("validate", string(req))

	_, err := types.ParseProperties(req)

	return err
}

func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	properties, err := types.ParseProperties(*spec.Properties)
	if err != nil {
		return nil, err
	}

	metadata, err := types.ParseMetadata(spec)
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

	api, err := p.API()
	if err != nil {
		return nil, err
	}

	if err = api.CreateInstance(name, &gcloud.InstanceSettings{
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

	if properties.TargetPool != "" {
		if err = api.AddInstanceToTargetPool(properties.TargetPool, name); err != nil {
			return nil, err
		}
	}

	return &id, nil
}

func (p *plugin) Destroy(id instance.ID) error {
	api, err := p.API()
	if err != nil {
		return err
	}

	err = api.DeleteInstance(string(id))

	log.Debugln("destroy", id, "err=", err)

	return err
}

func (p *plugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)

	api, err := p.API()
	if err != nil {
		return nil, err
	}

	instances, err := api.ListInstances()
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

		// When pets are deleted, we keep the disk
		if len(inst.Disks) > 0 && !inst.Disks[0].AutoDelete {
			id := instance.LogicalID(inst.Name)
			description.LogicalID = &id
		}

		result = append(result, description)
	}

	log.Debugln("matching count:", len(result))

	return result, nil
}
