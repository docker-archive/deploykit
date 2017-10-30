package vmwscript

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

//var log = logutil.New("module", "x/vmwscript")
//var debugV = logutil.V(200) // 100-500 are for typical debug levels, > 500 for highly repetitive logs (e.g. from polling)

// VCenterLogin - This function will use the VMware vCenter API to connect to a remote vCenter
func VCenterLogin(ctx context.Context, vm VMConfig) (*govmomi.Client, error) {
	// Parse URL from string
	u, err := url.Parse(*vm.VCenterURL)
	if err != nil {
		return nil, errors.New("URL can't be parsed, ensure it is https://username:password/<address>/sdk")
	}

	// Connect and log in to ESX or vCenter
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, fmt.Errorf("Error logging into vCenter, check address and credentials\nClient Error: %v", err)
	}
	return client, nil
}

// Provision - This does the initial provisioning
func Provision(ctx context.Context, client *govmomi.Client, vm VMConfig, inputTemplate string, outputName string) (*object.VirtualMachine, error) {

	f := find.NewFinder(client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, *vm.DCName)
	if err != nil {
		return nil, fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Find Datastore/Network
	datastore, err := f.DatastoreOrDefault(ctx, *vm.DSName)
	if err != nil {
		return nil, fmt.Errorf("Datastore [%s], could not be found", *vm.DSName)
	}

	dcFolders, err := dc.Folders(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error locating default datacenter folder")
	}

	// Set the host that the VM will be created on
	hostSystem, err := f.HostSystemOrDefault(ctx, *vm.VSphereHost)
	if err != nil {
		return nil, fmt.Errorf("vSphere host [%s], could not be found", *vm.VSphereHost)
	}

	// Find the resource pool attached to this host
	resourcePool, err := hostSystem.ResourcePool(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error locating default resource pool")
	}

	// Use finder for VM template
	vmTemplate, err := f.VirtualMachine(ctx, inputTemplate)
	if err != nil {
		return nil, err
	}

	pool := resourcePool.Reference()
	host := hostSystem.Reference()
	ds := datastore.Reference()

	// TODO - Allow modifiable relocateSpec for other DataStores
	relocateSpec := types.VirtualMachineRelocateSpec{
		Pool:      &pool,
		Host:      &host,
		Datastore: &ds,
	}

	// The only change we make to the Template Spec, is the config sha and group name
	spec := types.VirtualMachineConfigSpec{
		Annotation: "Built by InfraKit vmwscript for VMware",
	}

	// Changes can be to spec or relocateSpec
	cisp := types.VirtualMachineCloneSpec{
		Config:   &spec,
		Location: relocateSpec,
		Template: false,
		PowerOn:  true,
	}
	log.Info("Cloning a New Virtual Machine")
	vmObj := object.NewVirtualMachine(client.Client, vmTemplate.Reference())

	task, err := vmObj.Clone(ctx, dcFolders.VmFolder, outputName, cisp)
	if err != nil {
		return nil, errors.New("Creating new VM failed, more detail can be found in vCenter tasks")
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("Creating new VM failed\n%v", err)
	}

	if info.Error != nil {
		return nil, fmt.Errorf("Clone task finished with error: %s", info.Error)
	}

	clonedVM := object.NewVirtualMachine(client.Client, info.Result.(types.ManagedObjectReference))

	devices, _ := clonedVM.Device(ctx)

	net := devices.Find("ethernet-0")
	if net == nil {
		return nil, fmt.Errorf("Ethernet device does not exist on Template")
	}
	currentBacking := net.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()

	newNet, err := f.NetworkOrDefault(ctx, *vm.NetworkName)
	if err != nil {
		log.Crit("Network [%s], could not be found", *vm.NetworkName)
	}

	backing, err := newNet.EthernetCardBackingInfo(ctx)
	if err != nil {
		log.Crit("Unable to determine vCenter network backend\n%v", err)
	}

	netDev, err := object.EthernetCardTypes().CreateEthernetCard("vmxnet3", backing)
	if err != nil {
		log.Crit("Unable to create vmxnet3 network interface\n%v", err)
	}

	newBacking := netDev.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()

	currentBacking.Backing = newBacking.Backing
	log.Info("Modifying Networking backend")
	clonedVM.EditDevice(ctx, net)

	log.Info("Waiting for VMware Tools and Network connectivity...")
	guestIP, err := clonedVM.WaitForIP(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("New Virtual Machine has started with IP [%s]", guestIP)
	return clonedVM, nil
}

//RunTasks - This kicks of the running of VMware tasks
func RunTasks(ctx context.Context, client *govmomi.Client) {
	taskCount := DeploymentCount()
	vm := VMwareConfig() //Pull VMware configuration from JSON
	for i := 0; i < taskCount; i++ {
		task := NextDeployment()

		if task != nil {
			log.Info("Beginning Task [%s]: %s", task.Name, task.Note)

			newVM, err := Provision(ctx, client, *vm, task.Task.InputTemplate, task.Task.OutputName)

			if err != nil {
				log.Info("Provisioning has failed =>")
				log.Crit("%v", err)
			}

			auth := &types.NamePasswordAuthentication{
				Username: *vm.VMTemplateAuth.Username,
				Password: *vm.VMTemplateAuth.Password,
			}

			runCommands(ctx, client, newVM, auth, task)
			if task.Task.OutputType == "Template" {
				log.Info("Provisioning tasks have completed, powering down Virtual Machine (120 second Timeout)")

				err = newVM.ShutdownGuest(ctx)
				if err != nil {
					log.Info("Power Off task failed =>")
					log.Crit("%v", err)
				}
				for i := 1; i <= 120; i++ {
					state, err := newVM.PowerState(ctx)
					if err != nil {
						log.Crit("%v", err)
					}
					if state != types.VirtualMachinePowerStatePoweredOff {
						fmt.Printf("\r\033[36mWaiting for\033[m %d Seconds for VM Shutdown", i)
					} else {
						fmt.Printf("\r\033[32mShutdown completed in\033[m %d Seconds        \n", i)
						break
					}
					time.Sleep(1 * time.Second)
				}
				err = newVM.MarkAsTemplate(ctx)
				if err != nil {
					log.Info("Marking as Template has failed =>")
					log.Crit("%v", err)
				}
			}
		}
	}
}

func runCommands(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, deployment *DeploymentTask) {
	cmdCount := CommandCount(deployment)
	log.Info("%d commands will be executed.", cmdCount)
	for i := 0; i < cmdCount; i++ {
		cmd := NextCommand(deployment)
		// if cmd == nil then no more commands to run
		if cmd != nil {
			if cmd.CMDNote != "" { // If the command has a note, then print it out
				log.Info("Task: %s", cmd.CMDNote)
			}
			switch cmd.CMDType {
			case "execute":
				var err error
				var pid int64
				if cmd.CMDkey != "" {
					log.Info("Executing command from key [%s]", cmd.CMDkey)
					execKey := cmdResults[cmd.CMDkey]
					pid, err = vmExec(ctx, client, vm, auth, execKey, cmd.CMDUser)
				} else {
					pid, err = vmExec(ctx, client, vm, auth, cmd.CMD, cmd.CMDUser)
				}
				if err != nil {
					log.Crit("%v", err)
				}
				if cmd.CMDIgnore == false {
					err = watchPid(ctx, client, vm, auth, []int64{pid})
					if err != nil {
						log.Crit("%v", err)
					}
				}
			case "download":
				err := vmDownloadFile(ctx, client, vm, auth, cmd.CMDFilePath, cmd.CMDresultKey, cmd.CMDDelete)
				if err != nil {
					fmt.Printf("Error\n")
					log.Crit("%v", err)
				}
			}
			// Execute the command on the Virtual Machine
		}
	}
	ResetCounter()
}

func vmExec(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, command string, user string) (int64, error) {
	o := guest.NewOperationsManager(client.Client, vm.Reference())
	pm, _ := o.ProcessManager(ctx)

	sudoPath := "/bin/sudo" //TODO: This should perhaps be configurable incase some Distro has sudo in a weird place.

	// Add User to the built command
	var builtPath string
	if user != "" {
		builtPath = fmt.Sprintf("-n -u %s %s", user, command)
	} else {
		builtPath = fmt.Sprintf("-n %s", command)
	}

	cmdSpec := types.GuestProgramSpec{
		ProgramPath: sudoPath,
		Arguments:   builtPath,
	}

	pid, err := pm.StartProgram(ctx, auth, &cmdSpec)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func readEnv(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, path string, args string) error {
	o := guest.NewOperationsManager(client.Client, vm.Reference())
	pm, _ := o.ProcessManager(ctx)

	test, err := pm.ReadEnvironmentVariable(ctx, auth, []string{"swarm"})
	if err != nil {
		return err
	}
	fmt.Printf("%s", test)
	return nil
}

// This will download a file from the Virtual Machine to the localhost
func vmDownloadFile(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, path string, key string, deleteonDownload bool) error {
	o := guest.NewOperationsManager(client.Client, vm.Reference())
	fm, _ := o.FileManager(ctx)
	fileDetails, err := fm.InitiateFileTransferFromGuest(ctx, auth, path)
	if err != nil {
		return err
	}

	dl := soap.DefaultDownload

	e, err := client.ParseURL(fileDetails.Url)
	if err != nil {
		return err
	}

	f, _, err := client.Download(e, &dl)
	if err != nil {
		return err
	}
	// This will change to allow us to store contents of the filesystem in memory
	//_, err = io.Copy(os.Stdout, f)

	if key != "" {
		body, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		convertedString := string(body)
		cmdResults[key] = convertedString
	}

	log.Info("%d of file [%s] downloaded succesfully", fileDetails.Size, fileDetails.Url)
	log.Info("Removing file [%s] from Virtual Machine", path)
	if deleteonDownload == true {
		err = fm.DeleteFile(ctx, auth, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func watchPid(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, pid []int64) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	o := guest.NewOperationsManager(client.Client, vm.Reference())
	pm, _ := o.ProcessManager(ctx)

	process, err := pm.ListProcesses(ctx, auth, pid)
	if err != nil {
		return err
	}
	if len(process) > 0 {
		log.Info("Watching process [%d] cmd [%s]", process[0].Pid, process[0].CmdLine)
	} else {
		log.Crit("Process couldn't be found running")
	}

	// Counter if VMtools loses a previously watched process
	processTimeout := 0
	var counter int
	for {
		time.Sleep(1 * time.Second)
		process, err = pm.ListProcesses(ctx, auth, pid)

		if err != nil {
			return err
		}
		// Watch Process
		if process[0].EndTime == nil {
			fmt.Printf("\r\033[36mWatching for\033[m %d Seconds", counter)
			counter++
		} else {
			if process[0].ExitCode != 0 {
				fmt.Printf("\n")
				log.Info("Return code was not zero, please investigate logs on the Virtual Machine")
				break
			} else {
				fmt.Printf("\r\033[32mTask completed in\033[m %d Seconds\n", counter)
				return nil
			}
		}
		// Process, now can't be found...
		if len(process) == 0 {
			fmt.Printf("x")
			processTimeout++
			if processTimeout == 12 { // 12x5 seconds == 60 second time out
				fmt.Printf("\n")
				log.Info("Process no longer watched, VMware Tools may have been restarted")
				break
			}
		}
	}
	return nil
}
