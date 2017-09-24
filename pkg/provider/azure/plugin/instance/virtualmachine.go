package instance

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/go-autorest/autorest"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log    = logutil.New("module", "azure/instance")
	debugV = logutil.V(500)
)

// NewVirtualMachinePlugin returns a typed instance plugin
func NewVirtualMachinePlugin(options Options) instance.Plugin {
	client := compute.NewVirtualMachinesClient(options.SubscriptionID)
	client.Authorizer = autorest.NewBearerAuthorizer(options)
	return &virtualMachinePlugin{
		virtualMachinesAPI: &virtualMachinesClient{
			client: &client,
		},
		options: options,
	}
}

type virtualMachinesAPI interface {
	createOrUpdate(resourceGroupName, name string, vm compute.VirtualMachine) (<-chan compute.VirtualMachine, <-chan error)
	list(resourceGroupName string) (compute.VirtualMachineListResult, error)
	next(compute.VirtualMachineListResult) (compute.VirtualMachineListResult, error)
	get(resourceGroupName, name string) (compute.VirtualMachine, error)
	delete(resourceGroupName, name string) (<-chan compute.OperationStatusResponse, <-chan error)
}

type virtualMachinesClient struct {
	client *compute.VirtualMachinesClient
}

func (v *virtualMachinesClient) createOrUpdate(resourceGroupName, name string,
	vm compute.VirtualMachine) (<-chan compute.VirtualMachine, <-chan error) {
	return v.client.CreateOrUpdate(resourceGroupName, name, vm, nil)
}

func (v *virtualMachinesClient) list(resourceGroupName string) (compute.VirtualMachineListResult, error) {
	return v.client.List(resourceGroupName)
}

func (v *virtualMachinesClient) next(r compute.VirtualMachineListResult) (compute.VirtualMachineListResult, error) {
	return v.client.ListNextResults(r)
}

func (v *virtualMachinesClient) get(resourceGroupName, name string) (compute.VirtualMachine, error) {
	return v.client.Get(resourceGroupName, name, compute.InstanceView)
}

func (v *virtualMachinesClient) delete(resourceGroupName, name string) (<-chan compute.OperationStatusResponse, <-chan error) {
	return v.client.Delete(resourceGroupName, name, nil)
}

type virtualMachinePlugin struct {
	virtualMachinesAPI
	options Options
}

// Validate performs local validation on a provision request.
func (p *virtualMachinePlugin) Validate(req *types.Any) error {
	log.Debug("Validate", "req", req)
	vm := compute.VirtualMachine{}
	return req.Decode(&vm)
}

// Provision creates a new instance based on the spec.
func (p *virtualMachinePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	log.Debug("Provision", spec, "V", debugV)

	vm := &virtualMachine{}
	if spec.Properties == nil {
		return nil, fmt.Errorf("missing properties")
	}

	err := spec.Properties.Decode(vm)
	if err != nil {
		return nil, err
	}

	vm.mergeTags(spec.Tags, p.options.Namespace).addInit(spec.Init)

	vmName := fmt.Sprintf("vm-%v", randomSuffix(8))
	vmChan, errChan := p.virtualMachinesAPI.createOrUpdate(
		p.options.ResourceGroupName, vmName, compute.VirtualMachine(*vm))

	provisioned := <-vmChan
	err = <-errChan

	var instanceID *instance.ID
	if provisioned.ID != nil {
		// Azure API always uses vmName so we can just return this as the instance ID
		// so that we have a handle on future api calls
		id := instance.ID(vmName)
		instanceID = &id
	}

	return instanceID, err
}

// Label labels the instance
func (p *virtualMachinePlugin) Label(instance instance.ID, labels map[string]string) error {
	log.Debug("Label", "instance", instance, "labels", labels, "V", debugV)

	v, err := p.virtualMachinesAPI.get(p.options.ResourceGroupName, string(instance))
	if err != nil {
		return err
	}

	vm := virtualMachine(v).mergeTags(labels, p.options.Namespace)

	_, errChan := p.virtualMachinesAPI.createOrUpdate(
		p.options.ResourceGroupName, string(instance), compute.VirtualMachine(*vm))

	return <-errChan
}

// Destroy terminates an existing instance.
func (p *virtualMachinePlugin) Destroy(instance instance.ID, context instance.Context) error {
	log.Debug("Destroy", "instance", instance, "context", context, "V", debugV)

	_, errChan := p.virtualMachinesAPI.delete(p.options.ResourceGroupName, string(instance))
	return <-errChan
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// The properties flag indicates the client is interested in receiving details about each instance.
func (p *virtualMachinePlugin) DescribeInstances(labels map[string]string,
	properties bool) ([]instance.Description, error) {
	log.Debug("DescribeInstances", "labels", labels, "V", debugV)

	matches := []instance.Description{}

	all, err := p.virtualMachinesAPI.list(p.options.ResourceGroupName)
	if err != nil {
		return matches, err
	}
	if all.Value != nil {
		desc, err := vms(*all.Value).filter(labels).descriptions()
		if err != nil {
			return matches, err
		}
		matches = append(matches, desc...)
	}
	for all.NextLink != nil {
		all, err = p.virtualMachinesAPI.next(all)
		if err != nil {
			return matches, err
		}
		if all.Value != nil {
			desc, err := vms(*all.Value).filter(labels).descriptions()
			if err != nil {
				return matches, err
			}
			matches = append(matches, desc...)
		}
	}

	return matches, nil
}

type vms []compute.VirtualMachine

func (v vms) filter(labels map[string]string) vms {
	filtered := vms{}
	for _, vm := range v {
		if hasDifferentTags(labels, asTagMap(vm.Tags)) {
			continue
		}
		filtered = append(filtered, vm)
	}
	return filtered
}

func (v vms) descriptions() ([]instance.Description, error) {
	descriptions := []instance.Description{}
	for _, vm := range v {

		if vm.ID == nil {
			continue
		}

		props, err := types.AnyValue(v)
		if err != nil {
			return nil, err
		}
		desc := instance.Description{
			ID:         instance.ID(*vm.ID),
			LogicalID:  virtualMachine(vm).logicalID(),
			Properties: props,
			Tags:       asTagMap(vm.Tags),
		}
		descriptions = append(descriptions, desc)
	}
	return nil, nil
}

type virtualMachine compute.VirtualMachine

func (vm virtualMachine) logicalID() *instance.LogicalID {
	// Azure uses name throughout its api, so we just use the name as the logical ID.
	if vm.Name == nil {
		return nil
	}
	logical := instance.LogicalID(*vm.Name)
	return &logical
}

func (vm virtualMachine) mergeTags(a, b map[string]string) *virtualMachine {
	vmm := vm
	_, merged := mergeTags(asTagMap(vmm.Tags), a, b)
	vmm.Tags = formatTags(merged)
	return &vmm
}

func (vm virtualMachine) addInit(initStr string) *virtualMachine {
	vmm := vm
	init := initStr
	if vmm.OsProfile != nil {
		if vmm.OsProfile.CustomData != nil {
			init = *vmm.OsProfile.CustomData + "\n" + init
		}
		vmm.OsProfile.CustomData = &init
	} else {
		vmm.OsProfile = &compute.OSProfile{
			CustomData: &init,
		}
	}
	return &vmm
}
