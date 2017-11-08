package gcloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	logutil "github.com/docker/infrakit/pkg/log"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

const (
	// EnvProject is the environment variable that defines the default GCP project
	EnvProject = "CLOUDSDK_CORE_PROJECT"

	// EnvZone is the environment variable that defines the default GCP zone
	EnvZone = "CLOUDSDK_COMPUTE_ZONE"
)

// API is the list of operations that can execute on Google Cloud Platform.
type API interface {
	// GetProject returns the project name.
	GetProject() string

	// GetZone returns the zone short name.
	GetZone() string

	// ListInstances lists the instances.
	ListInstances() ([]*compute.Instance, error)

	// GetInstance find an instance by name.
	GetInstance(name string) (*compute.Instance, error)

	// CreateInstance creates an instance.
	CreateInstance(name string, settings *InstanceSettings) error

	// AddInstanceToTargetPool adds a list of instances to a target pool.
	AddInstanceToTargetPool(targetPool string, instances ...string) error

	// AddInstanceMetadata replaces/adds metadata items to an instance
	AddInstanceMetadata(instanceName string, items []*compute.MetadataItems) error

	// DeleteInstance deletes an instance.
	DeleteInstance(name string) error

	// DeleteInstanceGroupManager deletes an instance group manager.
	DeleteInstanceGroupManager(name string) error

	// DeleteInstanceTemplate deletes an instance template.
	DeleteInstanceTemplate(name string) error

	// ListInstanceGroupInstances lists the instances of an instance group found by its name.
	ListInstanceGroupInstances(name string) ([]*compute.InstanceWithNamedPorts, error)

	// CreateInstanceTemplate creates an instance template
	CreateInstanceTemplate(name string, settings *InstanceSettings) error

	// CreateInstanceGroupManager creates an instance group manager.
	CreateInstanceGroupManager(name string, settings *InstanceManagerSettings) error

	// SetInstanceTemplate sets the instance template used by a group manager.
	SetInstanceTemplate(name string, templateName string) error

	// ResizeInstanceGroupManager changes the target size of an instance group manager.
	ResizeInstanceGroupManager(name string, targetSize int64) error
}

// InstanceSettings lists the characteristics of a VM instance.
type InstanceSettings struct {
	Description string
	MachineType string
	Network     string
	Subnetwork  string
	PrivateIP   string
	Tags        []string
	Scopes      []string
	Disks       []DiskSettings
	Preemptible bool
	MetaData    []*compute.MetadataItems
}

// DiskSettings lists the characteristics of an attached disk.
type DiskSettings struct {
	Boot          bool
	Type          string
	Mode          string
	SizeGb        int64
	Image         string
	AutoDelete    bool
	ReuseExisting bool
	NameSuffix    string
}

// InstanceManagerSettings the characteristics of a VM instance template manager.
type InstanceManagerSettings struct {
	Description      string
	TemplateName     string
	TargetSize       int64
	TargetPools      []string
	BaseInstanceName string
}

type computeServiceWrapper struct {
	project string
	zone    string
	service *compute.Service
}

var log = logutil.New("module", "provider/google")

// NewAPI creates a new API instance.
func NewAPI(project, zone string) (API, error) {
	if project == "" {
		log.Debug("Project not passed on the command line", "project", project)

		project = findProject()
		if project == "" {
			return nil, errors.New("Missing project")
		}
	}

	if zone == "" {
		log.Debug("Zone not passed on the command line")

		zone = findZone()
		if zone == "" {
			return nil, errors.New("Missing zone")
		}
	}

	log.Debug("Project:", "project", project)
	log.Debug("Zone:", "zone", zone)

	serviceProvider := func() (*compute.Service, error) {
		client, err := google.DefaultClient(context.TODO(), compute.ComputeScope)
		if err != nil {
			return nil, err
		}

		return compute.New(client)
	}

	// Check that everything works
	service, err := serviceProvider()
	if err != nil {
		return nil, err
	}

	return &computeServiceWrapper{
		project: project,
		zone:    zone,
		service: service,
	}, nil
}

func findProject() string {
	if metadata.OnGCE() {
		log.Debug("- Query the metadata server...")

		projectID, err := metadata.ProjectID()
		if err == nil {
			return projectID
		}
	}

	log.Debug(" - Look for env var", "project", EnvProject)

	value, found := os.LookupEnv(EnvProject)
	if found && value != "" {
		return value
	}

	return ""
}

func findZone() string {
	if metadata.OnGCE() {
		log.Debug("- Query the metadata server...")

		zone, err := metadata.Zone()
		if err == nil {
			return zone
		}
	}

	log.Debug(" - Look for env var", "zone", EnvZone)

	value, found := os.LookupEnv(EnvZone)
	if found && value != "" {
		return value
	}

	return ""
}

func (g *computeServiceWrapper) GetProject() string {
	return g.project
}

func (g *computeServiceWrapper) GetZone() string {
	return g.zone
}

func (g *computeServiceWrapper) ListInstances() ([]*compute.Instance, error) {
	items := []*compute.Instance{}

	pageToken := ""
	for {
		list, err := g.service.Instances.List(g.project, g.zone).PageToken(pageToken).Do()
		if err != nil {
			return nil, err
		}

		for i := range list.Items {
			items = append(items, list.Items[i])
		}

		pageToken = list.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return items, nil
}

func (g *computeServiceWrapper) GetInstance(name string) (*compute.Instance, error) {
	return g.service.Instances.Get(g.project, g.zone, name).Do()
}

func (g *computeServiceWrapper) addAPIUrlPrefix(value string, prefix string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, g.service.BasePath+prefix) {
		return value
	}
	if strings.HasPrefix(value, prefix) {
		return g.service.BasePath + value
	}
	return g.service.BasePath + prefix + value
}

func (g *computeServiceWrapper) CreateInstance(name string, settings *InstanceSettings) error {
	machineType := g.addAPIUrlPrefix(settings.MachineType, g.project+"/zones/"+g.zone+"/machineTypes/")
	network := g.addAPIUrlPrefix(settings.Network, g.project+"/global/networks/")
	subnetwork := g.addAPIUrlPrefix(settings.Subnetwork, g.project+"/regions/"+g.region()+"/subnetworks/")

	disks, err := g.attachedDisks(name, settings.Disks)
	if err != nil {
		return err
	}

	instance := &compute.Instance{
		Name:        name,
		Description: settings.Description,
		MachineType: machineType,
		Tags: &compute.Tags{
			Items: settings.Tags,
		},
		Disks: disks,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network:    network,
				Subnetwork: subnetwork,
				NetworkIP:  settings.PrivateIP,
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
		Scheduling: &compute.Scheduling{
			AutomaticRestart:  true,
			OnHostMaintenance: "MIGRATE",
			Preemptible:       settings.Preemptible,
		},
	}
	log.Debug("Creating instance", "instance", instance)

	return g.doCall(g.service.Instances.Insert(g.project, g.zone, instance))
}

func (g *computeServiceWrapper) attachedDisks(instanceName string, disksSettings []DiskSettings) ([]*compute.AttachedDisk, error) {
	disks := []*compute.AttachedDisk{}

	for _, diskSettings := range disksSettings {
		disk, err := g.attachedDisk(instanceName, diskSettings)
		if err != nil {
			return nil, err
		}

		disks = append(disks, disk)
	}

	return disks, nil
}

func (g *computeServiceWrapper) attachedDisk(instanceName string, settings DiskSettings) (*compute.AttachedDisk, error) {
	sourceImage := g.addAPIUrlPrefix(settings.Image, "")
	diskType := g.addAPIUrlPrefix(settings.Type, g.project+"/zones/"+g.zone+"/diskTypes/")

	disk := &compute.AttachedDisk{
		Boot:       settings.Boot,
		Mode:       settings.Mode,
		AutoDelete: settings.AutoDelete,
		Type:       "PERSISTENT",
	}

	// Each disk is prefixed by the instance name and an optional suffix.
	diskName := instanceName + settings.NameSuffix

	var existingDisk *compute.Disk
	if settings.ReuseExisting {
		log.Debug("Trying to reuse disk", diskName)

		disk, err := g.service.Disks.Get(g.project, g.zone, diskName).Do()
		if err != nil || disk == nil {
			log.Debug("Couldn't find existing disk", diskName)
		} else if disk.SourceImage != sourceImage {
			log.Debug("Found existing disk that uses a wrong image. Let's delete", diskName)
			if err := g.doCall(g.service.Disks.Delete(g.project, g.zone, disk.Name)); err != nil {
				return nil, err
			}
		} else {
			log.Debug("Found existing disk", diskName)
			existingDisk = disk
		}
	}

	if existingDisk != nil {
		disk.Source = existingDisk.SelfLink
	} else if settings.Image == "" {
		log.Debug("Creating standalone disk", diskName)

		if err := g.doCall(g.service.Disks.Insert(g.project, g.zone, &compute.Disk{
			Name:   diskName,
			SizeGb: settings.SizeGb,
			Type:   diskType,
		})); err != nil {
			return nil, err
		}

		disk.Source = "projects/" + g.project + "/zones/" + g.zone + "/disks/" + diskName
	} else {
		// Create the disk alongside the instance
		disk.InitializeParams = &compute.AttachedDiskInitializeParams{
			DiskName:    diskName,
			SourceImage: sourceImage,
			DiskSizeGb:  settings.SizeGb,
			DiskType:    diskType,
		}
	}

	return disk, nil
}

func (g *computeServiceWrapper) AddInstanceToTargetPool(targetPool string, instances ...string) error {
	references := []*compute.InstanceReference{}
	for _, instance := range instances {
		references = append(references, &compute.InstanceReference{
			Instance: fmt.Sprintf("projects/%s/zones/%s/instances/%s", g.project, g.zone, instance),
		})
	}

	request := &compute.TargetPoolsAddInstanceRequest{
		Instances: references,
	}

	return g.doCall(g.service.TargetPools.AddInstance(g.project, g.region(), targetPool, request))
}

func (g *computeServiceWrapper) AddInstanceMetadata(instanceName string, items []*compute.MetadataItems) error {
	instance, err := g.GetInstance(instanceName)
	if err != nil {
		return err
	}

	for _, item := range items {
		found := false
		for _, existingItem := range instance.Metadata.Items {
			if existingItem.Key == item.Key {
				existingItem.Value = item.Value
				found = true
				break
			}
		}

		if !found {
			instance.Metadata.Items = append(instance.Metadata.Items, item)
		}

	}

	return g.doCall(g.service.Instances.SetMetadata(g.project, g.zone, instanceName, instance.Metadata))
}

func (g *computeServiceWrapper) DeleteInstance(name string) error {
	return g.doCall(g.service.Instances.Delete(g.project, g.zone, name))
}

func (g *computeServiceWrapper) DeleteInstanceGroupManager(name string) error {
	return g.doCall(g.service.InstanceGroupManagers.Delete(g.project, g.zone, name))
}

func (g *computeServiceWrapper) DeleteInstanceTemplate(name string) error {
	return g.doCall(g.service.InstanceTemplates.Delete(g.project, name))
}

func (g *computeServiceWrapper) ListInstanceGroupInstances(name string) ([]*compute.InstanceWithNamedPorts, error) {
	items := []*compute.InstanceWithNamedPorts{}

	pageToken := ""
	for {
		instances, err := g.service.InstanceGroups.ListInstances(g.project, g.zone, name, &compute.InstanceGroupsListInstancesRequest{
			InstanceState: "ALL",
		}).PageToken(pageToken).Do()
		if err != nil {
			return nil, err
		}

		for i := range instances.Items {
			items = append(items, instances.Items[i])
		}

		pageToken = instances.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return items, nil
}

func (g *computeServiceWrapper) CreateInstanceTemplate(name string, settings *InstanceSettings) error {
	network := g.addAPIUrlPrefix(settings.Network, g.project+"/global/networks/")
	subnetwork := g.addAPIUrlPrefix(settings.Subnetwork, g.project+"/regions/"+g.region()+"/subnetworks/")

	disks, err := g.attachedDisks(name, settings.Disks)
	if err != nil {
		return err
	}

	template := &compute.InstanceTemplate{
		Name:        name,
		Description: settings.Description,
		Properties: &compute.InstanceProperties{
			Description: settings.Description,
			MachineType: settings.MachineType,
			Tags: &compute.Tags{
				Items: settings.Tags,
			},
			Disks: disks,
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					Network:    network,
					Subnetwork: subnetwork,
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
			Scheduling: &compute.Scheduling{
				AutomaticRestart:  true,
				OnHostMaintenance: "MIGRATE",
				Preemptible:       settings.Preemptible,
			},
		},
	}

	return g.doCall(g.service.InstanceTemplates.Insert(g.project, template))
}

func (g *computeServiceWrapper) CreateInstanceGroupManager(name string, settings *InstanceManagerSettings) error {
	groupManager := &compute.InstanceGroupManager{
		Name:             name,
		Description:      settings.Description,
		Zone:             g.zone,
		InstanceTemplate: "projects/" + g.project + "/global/instanceTemplates/" + settings.TemplateName,
		BaseInstanceName: settings.BaseInstanceName,
		TargetPools:      settings.TargetPools,
		TargetSize:       settings.TargetSize,
	}

	return g.doCall(g.service.InstanceGroupManagers.Insert(g.project, g.zone, groupManager))
}

func (g *computeServiceWrapper) SetInstanceTemplate(name string, templateName string) error {
	request := &compute.InstanceGroupManagersSetInstanceTemplateRequest{
		InstanceTemplate: templateName,
	}

	return g.doCall(g.service.InstanceGroupManagers.SetInstanceTemplate(g.project, g.zone, name, request))
}

func (g *computeServiceWrapper) ResizeInstanceGroupManager(name string, targetSize int64) error {
	return g.doCall(g.service.InstanceGroupManagers.Resize(g.project, g.zone, name, targetSize))
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
