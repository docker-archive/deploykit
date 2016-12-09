package instance

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.gcp/plugin/instance/gcloud"
	"github.com/docker/infrakit/pkg/spi/instance"
)

const (
	defaultNamePrefix  = "instance"
	defaultMachineType = "g1-small"
	defaultNetwork     = "default"
	defaultDiskSizeMb  = 10
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

type instanceProperties struct {
	NamePrefix  string
	Description string
	MachineType string
	Network     string
	DiskSizeMb  int64
	Tags        []string
	Scopes      []string
	TargetPool  string
}

type gceInstance struct {
	instance.Description
}

type plugin struct {
	API func() (gcloud.GCloud, error)
}

// NewGCEInstancePlugin creates a new GCE instance plugin for a given project
// and zone.
func NewGCEInstancePlugin(project, zone string) instance.Plugin {
	log.Debugln("gce instance plugin. project=", project)

	return &plugin{
		API: func() (gcloud.GCloud, error) {
			return gcloud.New(project, zone)
		},
	}
}

func parseProperties(properties json.RawMessage) (*instanceProperties, error) {
	p := instanceProperties{}

	if err := json.Unmarshal(properties, &p); err != nil {
		return nil, err
	}

	if p.NamePrefix == "" {
		p.NamePrefix = defaultNamePrefix
	}
	if p.MachineType == "" {
		p.MachineType = defaultMachineType
	}
	if p.Network == "" {
		p.Network = defaultNetwork
	}
	if p.DiskSizeMb == 0 {
		p.DiskSizeMb = defaultDiskSizeMb
	}

	return &p, nil
}

func (p *plugin) Validate(req json.RawMessage) error {
	log.Debugln("validate", string(req))

	_, err := parseProperties(req)

	return err
}

func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	properties, err := parseProperties(*spec.Properties)
	if err != nil {
		return nil, err
	}

	var name string
	if spec.LogicalID != nil {
		name = string(*spec.LogicalID)
	} else {
		name = fmt.Sprintf("%s-%d", properties.NamePrefix, rand.Int63())
	}
	id := instance.ID(name)

	tags := make(map[string]string)
	for k, v := range spec.Tags {
		tags[k] = v
	}
	if spec.Init != "" {
		tags["startup-script"] = spec.Init
	}

	api, err := p.API()
	if err != nil {
		return nil, err
	}

	if err = api.CreateInstance(name, &gcloud.InstanceSettings{
		Description: properties.Description,
		MachineType: properties.MachineType,
		Network:     properties.Network,
		Tags:        properties.Tags,
		DiskSizeMb:  properties.DiskSizeMb,
		Scopes:      properties.Scopes,
		MetaData:    gcloud.TagsToMetaData(tags),
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

scan:
	for _, inst := range instances {
		instTags := gcloud.MetaDataToTags(inst.Metadata.Items)

		for k, v := range tags {
			if instTags[k] != v {
				continue scan // we implement AND
			}
		}

		logicalID := instance.LogicalID(inst.Name)
		result = append(result, instance.Description{
			LogicalID: &logicalID,
			ID:        instance.ID(inst.Name),
			Tags:      instTags,
		})
	}

	log.Debugln("matching count:", len(result))

	return result, nil
}
