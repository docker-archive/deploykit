package gcloud

import (
	"fmt"
	"strings"
	"time"

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

	// DeleteInstance deletes an instance
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

	return &computeServiceWrapper{
		service: service,
		project: project,
		zone:    zone,
	}, nil
}

func (g *computeServiceWrapper) ListInstances() ([]*compute.Instance, error) {
	list, err := g.service.Instances.List(g.project, g.zone).Do()
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (g *computeServiceWrapper) CreateInstance(name string, settings *InstanceSettings) error {
	instance := &compute.Instance{
		Name:        name,
		Description: settings.Description,
		MachineType: apiURL + g.project + "/zones/" + g.zone + "/machineTypes/" + settings.MachineType,
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
					SourceImage: apiURL + g.project + "/global/images/docker",
					DiskSizeGb:  settings.DiskSizeMb,
					DiskType:    apiURL + g.project + "/zones/" + g.zone + "/diskTypes/" + "pd-standard",
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: apiURL + g.project + "/global/networks/" + settings.Network,
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

func (g *computeServiceWrapper) DeleteInstance(name string) error {
	return g.doCall(g.service.Instances.Delete(g.project, g.zone, name))
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
