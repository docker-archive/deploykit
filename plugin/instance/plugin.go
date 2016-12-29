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
	defaultDiskImage   = "docker"
	defaultDiskType    = "pd-standard"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

type properties struct {
	NamePrefix  string
	Description string
	MachineType string
	Network     string
	DiskSizeMb  int64
	DiskImage   string
	DiskType    string
	Tags        []string
	Scopes      []string
	TargetPool  string
	Connect     bool
}

type plugin struct {
	API func() (gcloud.API, error)
}

// NewGCEInstancePlugin creates a new GCE instance plugin for a given project
// and zone.
func NewGCEInstancePlugin(project, zone string) instance.Plugin {
	gcePlugin := &plugin{
		API: func() (gcloud.API, error) {
			return gcloud.New(project, zone)
		},
	}

	_, err := gcePlugin.API()
	if err != nil {
		log.Fatal(err)
	}

	return gcePlugin
}

func parseProperties(rawJSON json.RawMessage) (*properties, error) {
	p := properties{}

	if err := json.Unmarshal(rawJSON, &p); err != nil {
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
	if p.DiskImage == "" {
		p.DiskImage = defaultDiskImage
	}
	if p.DiskType == "" {
		p.DiskType = defaultDiskType
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
	if properties.Connect {
		tags["serial-port-enable"] = "true"
	}

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
		AutoDeleteDisk:    spec.LogicalID == nil,
		ReuseExistingDisk: spec.LogicalID != nil,
		MetaData:          gcloud.TagsToMetaData(tags),
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

		// When pets are deleted, we keep the disk
		var logicalID *instance.LogicalID
		if !inst.Disks[0].AutoDelete {
			id := instance.LogicalID(inst.Name)
			logicalID = &id
		}

		result = append(result, instance.Description{
			LogicalID: logicalID,
			ID:        instance.ID(inst.Name),
			Tags:      instTags,
		})
	}

	log.Debugln("matching count:", len(result))

	return result, nil
}
