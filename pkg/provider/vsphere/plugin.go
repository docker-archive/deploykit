package vsphere

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"

	logutil "github.com/docker/infrakit/pkg/log"
)

var ignoreVMs bool

// Provides the logging capabilit
var log = logutil.New("module", "provider/oneview")

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

// This contains all of the information that the plugin needs to be aware of for provisioning and managing instances
type plugin struct {
	vC               *vCenter
	vCenterInternals *vcInternal
	fsm              []provisioningFSM
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

//miniFSM for managing the provisioning -> provisioned state
type provisioningFSM struct {
	timer        time.Time         // ideally will be a counter of minutes / seconds
	tags         map[string]string // tags that will be passed back per a describe function
	instanceName string            // name that we will use as a lookup to the actual backend that is provisioning
}

// NewInstancePlugin will take the cmdline/env configuration
func NewInstancePlugin(vc Options) instance.Plugin {
	return NewVSphereInstancePlugin(&vCenter{
		vCenterURL:  &vc.VCenterURL,
		dcName:      &vc.DataCenter,
		dsName:      &vc.DataStore,
		networkName: &vc.NetworkName,
		vSphereHost: &vc.VSphereHost,
	}, vc.IgnoreOnDestroy)
}

// NewVSphereInstancePlugin will take the cmdline/env configuration
func NewVSphereInstancePlugin(vc *vCenter, ignoreOnDestroy bool) instance.Plugin {
	// Set this as a global operation when working with vCenter, the plugin will need
	// restarting to modify this setting.
	ignoreVMs = ignoreOnDestroy
	// Attempt to log in to VMware vCenter and return the internal variables required
	internals, err := vCenterConnect(vc)
	if err != nil {
		// Exit with an error if we can't connect to vCenter (no point continuing)
		log.Crit("vCenter connection failure", "response", err.Error())
	}
	return &plugin{
		vC:               vc,
		vCenterInternals: &internals,
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-vSphere",
			Version: "0.6.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p *plugin) ExampleProperties() *types.Any {
	any, err := types.AnyValue(Spec{
		"exampleString": "a_string",
		"exampleBool":   true,
		"exampleInt":    1,
	})
	if err != nil {
		return nil
	}
	return any
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	log.Debug("validate", "Request", req.String())

	spec := Spec{}
	if err := req.Decode(&spec); err != nil {
		return err
	}

	log.Debug("Validated:", "Spec", spec)
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	var properties map[string]interface{}

	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, fmt.Errorf("Invalid instance properties: %s", err)
		}
	}

	newVM, err := parseParameters(properties, p)
	if err != nil {
		log.Error("Problems Whilst Parsting", "err", err)
		return nil, err
	}

	err = setInternalStructures(p.vC, p.vCenterInternals)
	if err != nil {
		log.Error("Problem whilst setting Internal Config", "err", err)
		return nil, err
	}

	if *p.vC.networkName != "" {
		findNetwork(p.vC, p.vCenterInternals)
	}

	// Use the VMware plugin data in order to provision a new VM server
	vmName := instance.ID(fmt.Sprintf(newVM.vmPrefix+"-%d", rand.Int63()))
	if spec.Tags != nil {
		log.Info("Provisioning", "vm", string(vmName), "group", spec.Tags[group.GroupTag])
	} else {
		log.Info("Provisioning", "vm", string(vmName))
	}

	//  Spawn a goroutine to provision in the background
	go func() {
		newVM.vmName = string(vmName)
		var newInstanceError error
		if newVM.vmTemplate != "" {
			log.Info("Cloning new instance", "template", newVM.vmTemplate)
			newInstanceError = cloneNewInstance(p, &newVM, spec)
		} else {
			newInstanceError = createNewVMInstance(p, &newVM, spec)
		}
		if newInstanceError != nil {
			log.Warn("Error creating", "vm", newVM.vmName)
			log.Error("vCenter problem", "err", newInstanceError)
		}
	}()

	var newInstance provisioningFSM
	newInstance.instanceName = string(vmName)
	newInstance.timer = time.Now().Add(time.Minute * 10) // Ten Minute timeout

	// duplicate the tags for the instance
	newInstance.tags = make(map[string]string)
	for k, v := range spec.Tags {
		newInstance.tags[k] = v
	}
	newInstance.tags["infrakit.state"] = "Provisioning"
	p.fsm = append(p.fsm, newInstance)

	log.Debug("FSM", "Count", len(p.fsm))

	return &vmName, nil
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	return fmt.Errorf("VMware vSphere VM label updates are not implemented yet")
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID, context instance.Context) error {
	log.Info(fmt.Sprintf("Currently running %s on instance: %v", context, instance))
	// Spawn a goroutine to delete in the background
	go func() {
		// TODO: Checks need adding to examine the instance that are quick enough not to trip timeout
		var err error
		if ignoreVMs == true {
			err = ignoreVM(p, string(instance))
		} else {
			err = deleteVM(p, string(instance))
		}
		if err != nil {
			log.Error("Destroying Instance failed", "err", err)
		}
	}()

	// TODO: Ideally the goroutine should return the error otherwise this function can never fail for InfraKit
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// TODO - need to define the fitlering of tags => AND or OR of matches?
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debug(fmt.Sprintf("describe-instances: %v", tags))
	results := []instance.Description{}

	groupName := tags[group.GroupTag]

	instances, err := findGroupInstances(p, groupName)
	if err != nil {
		log.Error("Problems finding group instances", "err", err)
	}

	// Iterate through group instances and find the sha from their annotation field
	for _, vmInstance := range instances {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		configSHA := returnDataFromVM(ctx, vmInstance, "sha")
		guestIP := returnDataFromVM(ctx, vmInstance, "guestIP")

		// Duplicate original tags
		vmTags := make(map[string]string)
		for k, v := range tags {
			vmTags[k] = v
		}

		vmTags[group.ConfigSHATag] = configSHA
		vmTags["guestIP"] = guestIP
		results = append(results, instance.Description{
			ID:        instance.ID(vmInstance.Name()),
			LogicalID: nil,
			Tags:      vmTags,
		})
	}
	log.Debug("Updating FSM", "Count", len(p.fsm))

	// DIFF what the endpoint is saying as reported versus what we have in the FSM
	var updatedFSM []provisioningFSM
	for _, unprovisionedInstance := range p.fsm {
		var provisioned bool

		for _, provisionedInstance := range results {

			if string(provisionedInstance.ID) == unprovisionedInstance.instanceName {
				provisioned = true
				// instance has been provisioned so break from loop
				break
			} else {
				provisioned = false
			}
		}
		if provisioned == false && unprovisionedInstance.timer.After(time.Now()) && unprovisionedInstance.tags[group.GroupTag] == tags[group.GroupTag] {
			updatedFSM = append(updatedFSM, unprovisionedInstance)
		}
	}

	p.fsm = make([]provisioningFSM, len(updatedFSM))
	copy(p.fsm, updatedFSM)

	log.Debug("FSM Updated", "Count", len(p.fsm))
	for _, unprovisionedInstances := range p.fsm {
		results = append(results, instance.Description{
			ID:        instance.ID(unprovisionedInstances.instanceName),
			LogicalID: nil,
			Tags:      unprovisionedInstances.tags,
		})
	}
	if len(results) == 0 {
		log.Info("No Instances found")
	}
	return results, nil
}
