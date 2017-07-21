package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/spi/instance"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	log "github.com/Sirupsen/logrus"
)

func findGroupInstances(p *plugin, groupName string) ([]*object.VirtualMachine, error) {
	// Without a groupName we have nothing to search for
	if groupName == "" {
		return nil, errors.New("The tag infrakit.group was blank")
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

	// Try to find virtual machines in the groupName folder (doesn't exist on ESXi)
	vmList, err := f.VirtualMachineList(ctx, groupName+"/*")

	if err == nil {
		if len(vmList) == 0 {
			log.Errorf("No Virtual Machines found in Folder")
		}
		return vmList, nil
	}
	// Restort to inspecting ALL VM Instances for group tags
	foundVMs, err := findInstancesFromAnnotation(ctx, f, groupName)
	return foundVMs, err
}

func returnDataFromVM(ctx context.Context, vmInstance *object.VirtualMachine, dataType string) string {
	var machineConfig mo.VirtualMachine
	if dataType == "guestIP" {
		err := vmInstance.Properties(ctx, vmInstance.Reference(), []string{"guest.ipAddress"}, &machineConfig)
		if err != nil {
			log.Errorf("%v", vmInstance)
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
			log.Errorf("%v", vmInstance)
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

func findInstancesFromAnnotation(ctx context.Context, f *find.Finder, groupName string) ([]*object.VirtualMachine, error) {
	var foundVMS []*object.VirtualMachine

	// Find ALL Virtual Machines
	vmList, err := f.VirtualMachineList(ctx, "*")
	if err != nil {
		return foundVMS, err
	}
	// Create new array of Virtual Machines that will hold all VMs that have the groupName
	for _, vmInstance := range vmList {
		instanceGroup := returnDataFromVM(ctx, vmInstance, "group")
		if instanceGroup == groupName {
			log.Debugf("%s matches groupName %s\n", vmInstance.Name(), groupName)
			foundVMS = append(foundVMS, vmInstance)
		}
	}
	return foundVMS, nil
}

func createNewVMInstance(p *plugin, vm *vmInstance, vmSpec instance.Spec) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spec := types.VirtualMachineConfigSpec{
		Name:       vm.vmName,
		GuestId:    "otherLinux64Guest",
		Files:      &types.VirtualMachineFileInfo{VmPathName: fmt.Sprintf("[%s]", p.vCenterInternals.datastore.Name())},
		NumCPUs:    int32(vm.vCpus),
		MemoryMB:   int64(vm.mem),
		Annotation: vmSpec.Tags["infrakit.group"] + "\n" + vmSpec.Tags["infrakit.config_sha"] + "\n" + vm.annotation,
	}

	scsi, err := object.SCSIControllerTypes().CreateSCSIController("pvscsi")
	if err != nil {
		return errors.New("Error creating pvscsi controller as part of new VM")
	}

	spec.DeviceChange = append(spec.DeviceChange, &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    scsi,
	})

	// Create a new finder that will discover the defaults and are looked for Networks/Datastores
	f := find.NewFinder(p.vCenterInternals.client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	groupFolder, err := f.Folder(ctx, vmSpec.Tags["infrakit.group"])
	if err != nil {
		log.Debugf("%v", err)
		groupFolder, err = p.vCenterInternals.dcFolders.VmFolder.CreateFolder(ctx, vmSpec.Tags["infrakit.group"])
		if err != nil {
			if err.Error() == "ServerFaultCode: The operation is not supported on the object." {
				baseFolder, _ := dc.Folders(ctx)
				groupFolder = baseFolder.VmFolder
			} else {
				if err.Error() == "ServerFaultCode: The name '"+vmSpec.Tags["infrakit.group"]+"' already exists." {
					return errors.New("A Virtual Machine exists with the same name as the InfraKit group")
				}
				log.Warnf("%v", err)
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
		log.Warnf("%v", err)
	}

	if *p.vC.networkName != "" {
		err = findNetwork(p.vC, p.vCenterInternals)
		if err != nil {
			log.Warnf("%v", err)
		} else {
			err = addNIC(newVM, p.vCenterInternals.network)
			if err != nil {
				log.Warnf("%v", err)
			}
		}
	}

	if vm.persistentSz > 0 {
		err = addVMDK(p, newVM, *vm)
		if err != nil {
			log.Warnf("%v", err)
		}
	}

	if vm.poweron == true {
		log.Infoln("Powering on provisioned Virtual Machine")
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
	dc, err := f.DefaultDatacenter(ctx)
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

	log.Infof("Adding VM Networking")
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

	log.Infof("Adding a persistent disk to the Virtual Machine")

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

	log.Debugln("Adding ISO to the Virtual Machine")

	if vm.AddDevice(ctx, add...); err != nil {
		return fmt.Errorf("Unable to add new CD-ROM device to VM configuration\n%v", err)
	}
	return nil
}
