package vsphere

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func cloneNewInstance(p *plugin, vm *vmInstance, vmSpec instance.Spec) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(p.vCenterInternals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *p.vC.dcName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Use finder for VM template
	vmTemplate, err := f.VirtualMachine(ctx, vm.vmTemplate)
	if err != nil {
		return err
	}
	pool := p.vCenterInternals.resourcePool.Reference()
	host := p.vCenterInternals.hostSystem.Reference()
	ds := p.vCenterInternals.datastore.Reference()

	// TODO - Allow modifiable relocateSpec for other DataStores
	relocateSpec := types.VirtualMachineRelocateSpec{
		Pool:      &pool,
		Host:      &host,
		Datastore: &ds,
	}

	// The only change we make to the Template Spec, is the config sha and group name
	spec := types.VirtualMachineConfigSpec{
		Annotation: vmSpec.Tags[group.GroupTag] + "\n" + vmSpec.Tags[group.ConfigSHATag] + "\n" + vm.annotation,
	}

	// Changes can be to spec or relocateSpec
	cisp := types.VirtualMachineCloneSpec{
		Config:   &spec,
		Location: relocateSpec,
		Template: false,
		PowerOn:  vm.poweron,
	}

	vmObj := object.NewVirtualMachine(p.vCenterInternals.client.Client, vmTemplate.Reference())

	var vmFolder *object.Folder

	// Check that a group has been submitted
	if vmSpec.Tags[group.GroupTag] == "" {
		vmFolder, err = f.DefaultFolder(ctx)
	} else {
		groupFolder, err := f.Folder(ctx, vmSpec.Tags[group.GroupTag])
		if err != nil {
			log.Warn("Issues finding a group folder", "err", err)
			groupFolder, err = p.vCenterInternals.dcFolders.VmFolder.CreateFolder(ctx, vmSpec.Tags[group.GroupTag])
			if err != nil {
				if err.Error() == "ServerFaultCode: The operation is not supported on the object." {
					baseFolder, _ := dc.Folders(ctx)
					groupFolder = baseFolder.VmFolder
				} else {
					if err.Error() == "ServerFaultCode: The name '"+vmSpec.Tags[group.GroupTag]+"' already exists." {
						return errors.New("A Virtual Machine exists with the same name as the InfraKit group")
					}
					log.Warn("Issues setting the group folder", "err", err)
				}
			}
		}
		vmFolder = groupFolder
	}

	task, err := vmObj.Clone(ctx, vmFolder, vm.vmName, cisp)
	if err != nil {
		return errors.New("Creating new VM failed, more detail can be found in vCenter tasks")
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return fmt.Errorf("Creating new VM failed\n%v", err)
	}
	if info.Error != nil {
		return fmt.Errorf("Clone task finished with error: %s", info.Error)
	}
	return nil
}

func createNewVMInstance(p *plugin, vm *vmInstance, vmSpec instance.Spec) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(p.vCenterInternals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *p.vC.dcName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	spec := types.VirtualMachineConfigSpec{
		Name:       vm.vmName,
		GuestId:    "otherLinux64Guest",
		Files:      &types.VirtualMachineFileInfo{VmPathName: fmt.Sprintf("[%s]", p.vCenterInternals.datastore.Name())},
		NumCPUs:    int32(vm.vCpus),
		MemoryMB:   int64(vm.mem),
		Annotation: vmSpec.Tags[group.GroupTag] + "\n" + vmSpec.Tags[group.ConfigSHATag] + "\n" + vm.annotation,
	}

	scsi, err := object.SCSIControllerTypes().CreateSCSIController("pvscsi")
	if err != nil {
		return errors.New("Error creating pvscsi controller as part of new VM")
	}

	spec.DeviceChange = append(spec.DeviceChange, &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    scsi,
	})

	groupFolder, err := f.Folder(ctx, vmSpec.Tags[group.GroupTag])
	if err != nil {
		log.Warn("Issues finding a group folder", "err", err)
		groupFolder, err = p.vCenterInternals.dcFolders.VmFolder.CreateFolder(ctx, vmSpec.Tags[group.GroupTag])
		if err != nil {
			if err.Error() == "ServerFaultCode: The operation is not supported on the object." {
				baseFolder, _ := dc.Folders(ctx)
				groupFolder = baseFolder.VmFolder
			} else {
				if err.Error() == "ServerFaultCode: The name '"+vmSpec.Tags[group.GroupTag]+"' already exists." {
					return errors.New("A Virtual Machine exists with the same name as the InfraKit group")
				}
				log.Warn("Issues setting the group folder", "err", err)
			}
		}
	}

	task, err := groupFolder.CreateVM(ctx, spec, p.vCenterInternals.resourcePool, p.vCenterInternals.hostSystem)
	if err != nil {
		return errors.New("Creating new VM failed, more detail can be found in vCenter tasks")
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return fmt.Errorf("Creating new VM failed\n%v", err)
	}

	// Retrieve the new VM
	newVM := object.NewVirtualMachine(p.vCenterInternals.client.Client, info.Result.(types.ManagedObjectReference))

	err = addISO(p, *vm, newVM)
	if err != nil {
		log.Warn(err.Error())
	}

	if *p.vC.networkName != "" {
		err = findNetwork(p.vC, p.vCenterInternals)
		if err != nil {
			log.Warn(err.Error())
		} else {
			err = addNIC(newVM, p.vCenterInternals.network)
			if err != nil {
				log.Warn(err.Error())
			}
		}
	}

	if vm.persistentSz > 0 {
		err = addVMDK(p, newVM, *vm)
		if err != nil {
			log.Warn(err.Error())
		}
	}

	if vm.poweron == true {
		log.Info("Powering on new Virtual Machine instance")
		return setVMPowerOn(true, newVM)
	}

	return nil
}

func deleteVM(p *plugin, vm string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(p.vCenterInternals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *p.vC.dcName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Try and find the virtual machine
	foundVM, err := f.VirtualMachine(ctx, vm)
	if err != nil {
		return fmt.Errorf("Error finding Virtual Machine\nClient Error => %v", err)
	}

	// Attempt to power off the VM regardless of power state
	_ = setVMPowerOn(false, foundVM)

	// If the Virtual Machine is found then we call Destroy against it.
	task, err := foundVM.Destroy(ctx)
	if err != nil {
		return errors.New("Delete operation has failed, more detail can be found in vCenter tasks")
	}

	_, err = task.WaitForResult(ctx, nil)
	if err != nil {
		return errors.New("Delete Task has failed, more detail can be found in vCenter tasks")
	}
	return nil
}

func ignoreVM(p *plugin, vm string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(p.vCenterInternals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *p.vC.dcName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Try and find the virtual machine
	foundVM, err := f.VirtualMachine(ctx, vm)
	if err != nil {
		return fmt.Errorf("Error finding Virtual Machine\nClient Error => %v", err)
	}

	configSpec := types.VirtualMachineConfigSpec{
		Annotation: "Deleted by InfraKit",
	}

	task, err := foundVM.Reconfigure(ctx, configSpec)
	if err != nil {
		return errors.New("Delete operation has failed, more detail can be found in vCenter tasks")
	}

	_, err = task.WaitForResult(ctx, nil)
	if err != nil {
		return errors.New("Delete Task has failed, more detail can be found in vCenter tasks")
	}
	return nil
}

func setVMPowerOn(state bool, vm *object.VirtualMachine) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var task *object.Task
	var err error

	if state == true {
		task, err = vm.PowerOn(ctx)
		if err != nil {
			return errors.New("Power On operation has failed, more detail can be found in vCenter tasks")
		}
	} else {
		task, err = vm.PowerOff(ctx)
		if err != nil {
			return errors.New("Power Off operation has failed, more detail can be found in vCenter tasks")
		}
	}
	_, err = task.WaitForResult(ctx, nil)
	if err != nil {
		return errors.New("Power On Task has failed, more detail can be found in vCenter tasks")
	}
	return nil
}

func addNIC(vm *object.VirtualMachine, net object.NetworkReference) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backing, err := net.EthernetCardBackingInfo(ctx)
	if err != nil {
		return fmt.Errorf("Unable to determine vCenter network backend\n%v", err)
	}

	netdev, err := object.EthernetCardTypes().CreateEthernetCard("vmxnet3", backing)
	if err != nil {
		return fmt.Errorf("Unable to create vmxnet3 network interface\n%v", err)
	}

	log.Info("Adding VM Networking")
	var add []types.BaseVirtualDevice
	add = append(add, netdev)

	if vm.AddDevice(ctx, add...); err != nil {
		return fmt.Errorf("Unable to add new networking device to VM configuration\n%v", err)
	}
	return nil
}

func addVMDK(p *plugin, vm *object.VirtualMachine, newVM vmInstance) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	devices, err := vm.Device(ctx)
	if err != nil {
		return fmt.Errorf("Unable to read devices from VM configuration\n%v", err)
	}

	controller, err := devices.FindDiskController("scsi")
	if err != nil {
		return fmt.Errorf("Unable to find SCSI device from VM configuration\n%v", err)
	}
	// The default is to have all persistent disks named linuxkit.vmdk
	disk := devices.CreateDisk(controller, p.vCenterInternals.datastore.Reference(), p.vCenterInternals.datastore.Path(fmt.Sprintf("%s/%s", newVM.vmName, newVM.vmName+".vmdk")))

	disk.CapacityInKB = int64(newVM.persistentSz * 1024)

	var add []types.BaseVirtualDevice
	add = append(add, disk)

	log.Info("Adding a persistent disk to the Virtual Machine")

	if vm.AddDevice(ctx, add...); err != nil {
		return fmt.Errorf("Unable to add new storage device to VM configuration\n%v", err)
	}
	return nil
}

func addISO(p *plugin, newInstance vmInstance, vm *object.VirtualMachine) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	devices, err := vm.Device(ctx)
	if err != nil {
		return fmt.Errorf("Unable to read devices from VM configuration\n%v", err)
	}

	ide, err := devices.FindIDEController("")
	if err != nil {
		return fmt.Errorf("Unable to find IDE device from VM configuration\n%v", err)
	}

	cdrom, err := devices.CreateCdrom(ide)
	if err != nil {
		return fmt.Errorf("Unable to create new CDROM device\n%v", err)
	}

	var add []types.BaseVirtualDevice
	add = append(add, devices.InsertIso(cdrom, p.vCenterInternals.datastore.Path(fmt.Sprintf("%s", newInstance.isoPath))))

	log.Debug("Adding ISO to the Virtual Machine")

	if vm.AddDevice(ctx, add...); err != nil {
		return fmt.Errorf("Unable to add new CD-ROM device to VM configuration\n%v", err)
	}
	return nil
}

func findGroupInstances(p *plugin, groupName string) ([]*object.VirtualMachine, error) { // Without a groupName we have nothing to search for
	if groupName == "" {
		return nil, fmt.Errorf("The tag %s was blank", group.GroupTag)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(p.vCenterInternals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *p.vC.dcName)
	if err != nil {
		return nil, fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// This will either hold all of the virtual machines in a folder, or ALL VMs
	var vmList []*object.VirtualMachine

	// Try to find virtual machines in the groupName folder (doesn't exist on ESXi)
	vmList, err = f.VirtualMachineList(ctx, groupName+"/*")

	// If there is an error, it should indicate that there is no folder
	if err != nil {
		// Resort to inspecting ALL VM Instances for group tags
		vmList, err = f.VirtualMachineList(ctx, "*")
		if err != nil {
			return vmList, err
		}
	}

	// Go through the VMs and find the correct ones from the groupname
	return findInstancesFromAnnotation(ctx, vmList, groupName)
}

func findInstancesFromAnnotation(ctx context.Context, vmList []*object.VirtualMachine, groupName string) ([]*object.VirtualMachine, error) {
	var foundVMS []*object.VirtualMachine

	// Find ALL Virtual Machines

	// Create new array of Virtual Machines that will hold all VMs that have the groupName
	for _, vmInstance := range vmList {
		instanceGroup := returnDataFromVM(ctx, vmInstance, "group")
		if instanceGroup == groupName {

			foundVMS = append(foundVMS, vmInstance)
		}
	}
	return foundVMS, nil
}

func returnDataFromVM(ctx context.Context, vmInstance *object.VirtualMachine, dataType string) string {
	var machineConfig mo.VirtualMachine
	if dataType == "guestIP" {
		err := vmInstance.Properties(ctx, vmInstance.Reference(), []string{"guest.ipAddress"}, &machineConfig)
		if err != nil {
			log.Error(err.Error())
		} else {
			// Ensure that we're pointing to a Guest Config struct
			if machineConfig.Guest != nil {
				//log.Printf("%v\n%s", vmInstance, machineConfig.Guest.IpAddress)
				return machineConfig.Guest.IpAddress
			}
		}
	} else {
		err := vmInstance.Properties(ctx, vmInstance.Reference(), []string{"config.annotation"}, &machineConfig)
		if err != nil {
			log.Error(err.Error())
		}
		// Ensure that we're pointing to a Configuration struct
		if machineConfig.Config != nil {
			annotationData := strings.Split(machineConfig.Config.Annotation, "\n")
			if len(annotationData) > 1 {
				switch dataType {
				case "group":
					return annotationData[0]
				case "sha":
					return annotationData[1]
				}
			}
		}
	}
	return ""
}
