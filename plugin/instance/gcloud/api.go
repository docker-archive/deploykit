package gcloud

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

const apiURL = "https://www.googleapis.com/compute/v1/projects/"

// GCloud is the list of operations that can execute on Google Cloud Platform.
type GCloud interface {
	// ListInstances lists the instances for a given zone.
	ListInstances() ([]*compute.Instance, error)

	// CreateInstance creates an instance.
	CreateInstance(name string, settings *InstanceSettings) error

	// AddInstanceToTargetPool adds a list of instances to a target pool.
	AddInstanceToTargetPool(targetPool string, instances ...string) error

	// DeleteInstance deletes an instance.
	DeleteInstance(name string) error
}

// InstanceSettings lists the characteristics of an VM instance.
type InstanceSettings struct {
	Description string
	MachineType string
	Network     string
	Tags        []string
	Scopes      []string
	DiskSizeMb  int64
	DiskImage   string
	DiskType    string
	MetaData    []*compute.MetadataItems
}

type computeServiceWrapper struct {
	service *compute.Service
	project string
	zone    string
}

// New creates a new Gcloud instance.
func New(project, zone string) (GCloud, error) {
	client, err := google.DefaultClient(oauth2.NoContext, compute.ComputeScope)
	if err != nil {
		return nil, err
	}

	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	// Try to find the project from the metaData server
	if project == "" {
		project, err = metadata("project/project-id")
		if err != nil {
			return nil, err
		}
	}
	if project == "" {
		return nil, errors.New("Missing project")
	}
	log.Debugln("Project:", project)

	// Try to find the zone from the metaData server
	if zone == "" {
		zoneURI, err := metadata("instance/zone")
		if err != nil {
			return nil, err
		}

		zone = last(zoneURI)
	}
	if zone == "" {
		return nil, errors.New("Missing zone")
	}
	log.Debugln("Zone:", zone)

	return &computeServiceWrapper{
		service: service,
		project: project,
		zone:    zone,
	}, nil
}

func metadata(path string) (string, error) {
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/"+path, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New(resp.Status)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (g *computeServiceWrapper) ListInstances() ([]*compute.Instance, error) {
	list, err := g.service.Instances.List(g.project, g.zone).Do()
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func addAPIUrlPrefix(value string, prefix string) string {
	if strings.HasPrefix(value, apiURL+prefix) {
		return value
	}
	if strings.HasPrefix(value, prefix) {
		return apiURL + value
	}
	return apiURL + prefix + value
}

func (g *computeServiceWrapper) CreateInstance(name string, settings *InstanceSettings) error {
	machineType := addAPIUrlPrefix(settings.MachineType, g.project+"/zones/"+g.zone+"/machineTypes/")
	network := addAPIUrlPrefix(settings.Network, g.project+"/global/networks/")
	sourceImage := addAPIUrlPrefix(settings.DiskImage, "")
	diskType := addAPIUrlPrefix(settings.DiskType, g.project+"/zones/"+g.zone+"/diskTypes/")

	instance := &compute.Instance{
		Name:        name,
		Description: settings.Description,
		MachineType: machineType,
		Tags: &compute.Tags{
			Items: settings.Tags,
		},
		Disks: []*compute.AttachedDisk{
			{
				Boot:       true,
				AutoDelete: true,
				Type:       "PERSISTENT",
				Mode:       "READ_WRITE",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    name + "-disk",
					SourceImage: sourceImage,
					DiskSizeGb:  settings.DiskSizeMb,
					DiskType:    diskType,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: network,
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
					},
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: settings.MetaData,
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email:  "default",
				Scopes: settings.Scopes,
			},
		},
	}

	return g.doCall(g.service.Instances.Insert(g.project, g.zone, instance))
}

func (g *computeServiceWrapper) AddInstanceToTargetPool(targetPool string, instances ...string) error {
	references := []*compute.InstanceReference{}
	for _, instance := range instances {
		references = append(references, &compute.InstanceReference{
			Instance: fmt.Sprintf("projects/%s/zones/%s/instances/%s", g.project, g.zone, instance),
		})
	}

	return g.doCall(g.service.TargetPools.AddInstance(g.project, g.region(), targetPool, &compute.TargetPoolsAddInstanceRequest{
		Instances: references,
	}))
}

func (g *computeServiceWrapper) DeleteInstance(name string) error {
	return g.doCall(g.service.Instances.Delete(g.project, g.zone, name))
}

func (g *computeServiceWrapper) region() string {
	return g.zone[:len(g.zone)-2]
}

// Call is an async Google Api call
type Call interface {
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
}

func (g *computeServiceWrapper) doCall(call Call) error {
	op, err := call.Do()
	if err != nil {
		return err
	}

	for {
		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("Operation error: %v", *op.Error.Errors[0])
			}

			return nil
		}

		time.Sleep(1 * time.Second)

		op, err = g.getOperationCall(op).Do()
		if err != nil {
			return err
		}
	}
}

func (g *computeServiceWrapper) getOperationCall(op *compute.Operation) Call {
	switch {
	case op.Zone != "":
		return g.service.ZoneOperations.Get(g.project, last(op.Zone), op.Name)
	case op.Region != "":
		return g.service.RegionOperations.Get(g.project, last(op.Region), op.Name)
	default:
		return g.service.GlobalOperations.Get(g.project, op.Name)
	}
}

func last(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
