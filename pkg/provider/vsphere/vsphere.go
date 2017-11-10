package vsphere

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

// Options capture the config parameters required to create the plugin
type Options struct {
	VCenterURL      string
	DataCenter      string
	DataStore       string
	NetworkName     string
	VSphereHost     string
	IgnoreOnDestroy bool
}

type vCenter struct {
	vCenterURL  *string
	dcName      *string
	dsName      *string
	networkName *string
	vSphereHost *string
}

type vcInternal struct {
	client       *govmomi.Client
	datastore    *object.Datastore
	dcFolders    *object.DatacenterFolders
	hostSystem   *object.HostSystem
	network      object.NetworkReference
	resourcePool *object.ResourcePool
}

type vmInstance struct {

	// Used with LinuxKit ISOs
	isoPath string

	// Used with a VMware VM template
	vmTemplate string
	// TODO - Add in reading template from one DS and deploy to another
	vmTemplateDatastore string

	// Used by InfraKit to track group
	groupTag string

	// Folder that will store all InfraKit instances
	instanceFolder string
	// InfraKit vSphere instance settings
	annotation   string
	vmPrefix     string
	vmName       string
	persistent   string
	persistentSz int
	vCpus        int
	mem          int
	poweron      bool
	guestIP      bool
}

func vCenterConnect(vc *vCenter) (vcInternal, error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var internals vcInternal
	// Parse URL from string
	u, err := url.Parse(*vc.vCenterURL)
	if err != nil {
		return internals, errors.New("URL can't be parsed, ensure it is https://username:password/<address>/sdk")
	}

	// Connect and log in to ESX or vCenter
	internals.client, err = govmomi.NewClient(ctx, u, true)
	if err != nil {
		return internals, err
	}
	return internals, nil
}

func setInternalStructures(vc *vCenter, internals *vcInternal) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(internals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *vc.dcName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Find Datastore/Network
	internals.datastore, err = f.DatastoreOrDefault(ctx, *vc.dsName)
	if err != nil {
		return fmt.Errorf("Datastore [%s], could not be found", *vc.dsName)
	}

	internals.dcFolders, err = dc.Folders(ctx)
	if err != nil {
		return fmt.Errorf("Error locating default datacenter folder")
	}

	// Set the host that the VM will be created on
	internals.hostSystem, err = f.HostSystemOrDefault(ctx, *vc.vSphereHost)
	if err != nil {
		return fmt.Errorf("vSphere host [%s], could not be found", *vc.vSphereHost)
	}

	// Find the resource pool attached to this host
	internals.resourcePool, err = internals.hostSystem.ResourcePool(ctx)
	if err != nil {
		return fmt.Errorf("Error locating default resource pool")
	}
	return nil
}

func parseParameters(properties map[string]interface{}, p *plugin) (vmInstance, error) {

	var newInstance vmInstance

	log.Debug("Building vCenter specific parameters")
	if *p.vC.vCenterURL == "" {
		if properties["vCenterURL"] == nil {
			return newInstance, errors.New("Environment variable VCURL or .yml vCenterURL must be set")
		}
		*p.vC.vCenterURL = properties["vCenterURL"].(string)
	}

	if properties["Datacenter"] == nil {
		log.Warn("The property 'Datacenter' hasn't been set the API will choose the default, which could cause errors in Linked-Mode")
	} else {
		*p.vC.dcName = properties["Datacenter"].(string)
	}

	if properties["Datastore"] == nil {
		return newInstance, errors.New("Property 'Datastore' must be set")
	}
	*p.vC.dsName = properties["Datastore"].(string)
	log.Debug("Setting datastore", "datastore", *p.vC.dsName)

	if properties["Hostname"] == nil {
		return newInstance, errors.New("Property 'Hostname' must be set")
	}
	*p.vC.vSphereHost = properties["Hostname"].(string)

	if properties["Network"] == nil {
		log.Warn("The property 'Network' hasn't been set, no networks will be attached to VM")
	} else {
		*p.vC.networkName = properties["Network"].(string)
	}

	if properties["Annotation"] != nil {
		newInstance.annotation = properties["Annotation"].(string)
	}

	if properties["vmPrefix"] == nil {
		newInstance.vmPrefix = "vm"
	} else {
		newInstance.vmPrefix = properties["vmPrefix"].(string)
	}

	if properties["isoPath"] == nil {
		log.Debug("The property 'isoPath' hasn't been set, bootable ISO will not be added to the VM")
	} else {
		newInstance.isoPath = properties["isoPath"].(string)
	}

	if properties["Template"] != nil {
		newInstance.vmTemplate = properties["Template"].(string)
	}

	if properties["CPUs"] == nil {
		newInstance.vCpus = 1
	} else {
		newInstance.vCpus = int(properties["CPUs"].(float64))

	}

	if properties["Memory"] == nil {
		newInstance.mem = 512
	} else {
		newInstance.mem = int(properties["Memory"].(float64))
	}

	if properties["persistantSZ"] == nil {
		newInstance.persistentSz = 0
	} else {
		newInstance.persistentSz = int(properties["persistantSZ"].(float64))
	}

	if properties["PowerOn"] == nil {
		newInstance.poweron = false
	} else {
		newInstance.poweron = bool(properties["PowerOn"].(bool))
	}

	return newInstance, nil
}

func findNetwork(vc *vCenter, internals *vcInternal) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(internals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *vc.dcName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	if *vc.networkName != "" {
		internals.network, err = f.NetworkOrDefault(ctx, *vc.networkName)
		if err != nil {
			return fmt.Errorf("Network [%s], could not be found", *vc.networkName)
		}
	}
	return nil
}
