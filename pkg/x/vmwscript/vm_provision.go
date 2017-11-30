package vmwscript

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// VCenterLogin - This function will use the VMware vCenter API to connect to a remote vCenter
func VCenterLogin(ctx context.Context, vm VMConfig) (*govmomi.Client, error) {
	// Parse URL from string
	u, err := url.Parse(vm.VCenterURL)
	if err != nil {
		return nil, errors.New("URL can't be parsed, ensure it is https://username:password/<address>/sdk")
	}

	// Connect and log in to ESX or vCenter
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Provision - This does the initial provisioning
func Provision(ctx context.Context, client *govmomi.Client, vm VMConfig, inputTemplate string, outputName string) (*object.VirtualMachine, error) {

	f := find.NewFinder(client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, vm.DCName)
	if err != nil {
		return nil, fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Find Datastore/Network
	datastore, err := f.DatastoreOrDefault(ctx, vm.DSName)
	if err != nil {
		return nil, fmt.Errorf("Datastore [%s], could not be found", vm.DSName)
	}

	dcFolders, err := dc.Folders(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error locating default datacenter folder")
	}

	// Set the host that the VM will be created on
	hostSystem, err := f.HostSystemOrDefault(ctx, vm.VSphereHost)
	if err != nil {
		return nil, fmt.Errorf("vSphere host [%s], could not be found", vm.VSphereHost)
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
		return nil, err
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

	newNet, err := f.NetworkOrDefault(ctx, vm.NetworkName)
	if err != nil {
		errorMessage := fmt.Sprintf("Network [%s], could not be found", vm.NetworkName)
		log.Crit(errorMessage)
	}

	backing, err := newNet.EthernetCardBackingInfo(ctx)
	if err != nil {
		errorMessage := fmt.Sprintf("Unable to determine vCenter network backend\n%v", err)
		log.Crit(errorMessage)
	}

	netDev, err := object.EthernetCardTypes().CreateEthernetCard("vmxnet3", backing)
	if err != nil {
		errorMessage := fmt.Sprintf("Unable to create vmxnet3 network interface\n%v", err)
		log.Crit(errorMessage)
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

	log.Info("New Virtual Machine has started its networking", "IP Address", guestIP)
	return clonedVM, nil
}

//RunTasks - This kicks of the running of VMware tasks
func (plan *DeploymentPlan) RunTasks(ctx context.Context, client *govmomi.Client) {
	taskCount := plan.DeploymentCount()
	vm := plan.VMWConfig
	for i := 0; i < taskCount; i++ {
		task := plan.NextDeployment()

		if task != nil {
			log.Info("Beginning deployment", "task", task.Name, "note", task.Note)
			newVM, err := Provision(ctx, client, vm, task.Task.InputTemplate, task.Task.OutputName)

			if err != nil {
				log.Error("Provisioning new Virtual Machine has failed")
				log.Crit(err.Error())
				os.Exit(-1)
			}

			auth := &types.NamePasswordAuthentication{
				Username: vm.VMTemplateAuth.Username,
				Password: vm.VMTemplateAuth.Password,
			}

			// Check if a networking configuration exists
			if task.Task.Network != nil {
				// Determine the distribution to configure the networking
				switch task.Task.Network.Distro {
				case "centos": // Set up networking for the CentOS distribution
					setCentosNetwork(ctx, client, newVM, auth, task.Task.Network)
					log.Info("Restarting Virtual Machine, and waiting for tools")
					newVM.RebootGuest(ctx)
					time.Sleep(time.Second * 10)
					var counter int
					for {
						time.Sleep(1 * time.Second)
						tools, err := newVM.IsToolsRunning(ctx)
						if err != nil {
							log.Crit("VMware tools", "err", err)
						}
						// Watch Virtual Machine for tools to start
						if tools == false {
							fmt.Printf("\r\033[36mWaiting for VMware Tools to start \033[m%d Seconds", counter)
							counter++
						} else {
							fmt.Printf("\r\033[32mVirtual Machine has succesfully restarted in\033[m %d Seconds\n", counter)
							time.Sleep(time.Second * 5) //TODO: This is here to allow Docker to start, a better method is needed
							break
						}
					}
					if err != nil {
						log.Error("Error during networking configuration", "err", err)
					}
				case "rhel": // Set up networking for the RHEL distribution
					log.Error("Unsupported Distribtion")
				case "ubuntu": // Set up networking for the Ubuntu distribution
					log.Error("Unsupported Distribtion")
				case "debian": // Set up networking for the Debian distribution
					log.Error("Unsupported Distribtion")
				case "windows": // Set up networking for the Windows Operating System
					log.Error("Unsupported OS")
				default: // return some 'unsupported' error.
					log.Error("Unsupported Distribtion")
				}

			}
			// Hand over to the funciton that will run through the array of commands
			plan.runCommands(ctx, client, newVM, auth, task)

			// Once tasks have been ran, determine what to do with the finished vm
			if task.Task.OutputType == "Template" {
				log.Info("Provisioning tasks have completed, powering down Virtual Machine (120 second Timeout)")

				err = newVM.ShutdownGuest(ctx)
				if err != nil {
					log.Error("Power Off task failed")
					log.Crit(err.Error())
					os.Exit(-1)
				}
				for i := 1; i <= 120; i++ {
					state, err := newVM.PowerState(ctx)
					if err != nil {
						log.Crit(err.Error())
						os.Exit(-1)
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
					log.Crit(err.Error())
					os.Exit(-1)
				}
			}
		}
	}
}

func (plan *DeploymentPlan) runCommands(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, deployment *DeploymentTask) {
	cmdCount := CommandCount(deployment)
	log.Info(fmt.Sprintf("%d commands will be executed.", cmdCount))
	for i := 0; i < cmdCount; i++ {
		cmd := plan.NextCommand(deployment)
		// if cmd == nil then no more commands to run
		if cmd != nil {
			if cmd.CMDNote != "" { // If the command has a note, then print it out
				log.Info(fmt.Sprintf("Task: \033[35m%s\033[m", cmd.CMDNote))
			}
			switch cmd.CMDType {
			case "execute":
				var err error
				var pid int64
				if cmd.CMDkey != "" {
					log.Info(fmt.Sprintf("Executing command from key [%s]", cmd.CMDkey))
					execKey := cmdResults[cmd.CMDkey]
					pid, err = vmExec(ctx, client, vm, auth, execKey, cmd.CMDUser)
				} else {
					pid, err = vmExec(ctx, client, vm, auth, cmd.CMD, cmd.CMDUser)
				}
				if err != nil {
					log.Error("Task failed", "err", err)
				}
				if cmd.CMDIgnore == false {
					err = watchPid(ctx, client, vm, auth, []int64{pid})
					if err != nil {
						log.Error("Watching Task failed", "err", err)
					}
				}
			case "download":
				err := vmDownloadFile(ctx, client, vm, auth, cmd.CMDFilePath, cmd.CMDresultKey, cmd.CMDDelete)
				if err != nil {
					fmt.Printf("Error\n")
					log.Error(err.Error())
				}
			}
			// Execute the command on the Virtual Machine
		}
	}
	plan.ResetCounter()
}

func vmExec(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, command string, user string) (int64, error) {

	if auth.Username == "" {
		return 0, fmt.Errorf("No VM Guest username set to run the VMTools")
	}
	if auth.Password == "" {
		return 0, fmt.Errorf("No VM Guest password set to run the VMTools")
	}

	o := guest.NewOperationsManager(client.Client, vm.Reference())
	pm, _ := o.ProcessManager(ctx)

	var progPath string

	// As vmtoolds doesn't execute against a TTY, we do some clever cli work to emulate either sudo or su
	var builtPath string
	if user != "" {
		progPath = "/bin/sudo" //TODO: This should perhaps be configurable incase some Distro has sudo in a weird place.
		builtPath = fmt.Sprintf("-n -u %s %s", user, command)
	} else {
		progPath = "/bin/su" //TODO: This should perhaps be configurable incase some Distro has su in a weird place.
		builtPath = fmt.Sprintf("-c \"%s\"", command)
	}

	cmdSpec := types.GuestProgramSpec{
		ProgramPath: progPath,
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

	log.Info("File download", "name", fileDetails.Url, "size", fileDetails.Size)
	if deleteonDownload == true {
		err = fm.DeleteFile(ctx, auth, path)
		if err != nil {
			return err
		}
		log.Info("Removed file", "name", path)
	}
	return nil
}

func watchPid(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, pid []int64) error {

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	o := guest.NewOperationsManager(client.Client, vm.Reference())
	pm, _ := o.ProcessManager(ctx)

	process, err := pm.ListProcesses(ctx, auth, pid)
	if err != nil {
		return err
	}
	if len(process) > 0 {
		// The command is hidden (may contain secure information)
		log.Debug("Watching command", "cmd", process[0].CmdLine)
	} else {
		log.Error("Process couldn't be found running")
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
			fmt.Printf("\r\033[36mWatching pid\033[m %d \033[36mfor\033[m %d Seconds", process[0].Pid, counter)
			counter++
		} else {
			if process[0].ExitCode != 0 {
				fmt.Printf("\n")
				log.Warn("Return code was not zero, please investigate logs on the Virtual Machine")
				break
			} else {
				fmt.Printf("\r\033[32mProcess completed successfully in\033[m %d Seconds\n", counter)
				return nil
			}
		}
		// Process, now can't be found...
		if len(process) == 0 {
			fmt.Printf("x")
			processTimeout++
			if processTimeout == 12 { // 12x5 seconds == 60 second time out
				fmt.Printf("\n")
				log.Warn("Process no longer watched, VMware Tools may have been restarted")
				break
			}
		}
	}
	return nil
}

// setCentosNetwork - CentOS only networking
func setCentosNetwork(ctx context.Context, client *govmomi.Client, vm *object.VirtualMachine, auth *types.NamePasswordAuthentication, netConfig *NetworkConfig) error {

	if netConfig.DeviceName != "" {
		log.Info("Configuring networking", "device", netConfig.DeviceName)
	} else {
		return fmt.Errorf("No Ethernet device to configure")
	}
	var pid int64
	var err error

	if netConfig.Address != "" {
		log.Warn("Removing existing networking configuration", "device", netConfig.DeviceName)
		pid, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli connection delete %s", netConfig.DeviceName), netConfig.SudoUser)

		if err != nil {
			log.Error("Setting Address failed", "err", err)
		}
		err = watchPid(ctx, client, vm, auth, []int64{pid})
		if err != nil {
			log.Error("Setting Address failed", "err", err)
		}
		if netConfig.Gateway != "" {
			log.Info("Configuring network", "ip", netConfig.Address, "gateway", netConfig.Gateway)
			pid, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli connection add type ethernet con-name InfraKit ifname %s ip4 %s gw4 %s", netConfig.DeviceName, netConfig.Address, netConfig.Gateway), netConfig.SudoUser)
		} else {
			log.Info("Configuring network", "ip", netConfig.Address)
			pid, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli connection add type ethernet con-name InfraKit ifname %s ip4 %s", netConfig.DeviceName, netConfig.Address), netConfig.SudoUser)
		}
		if err != nil {
			log.Error("Setting Address failed", "err", err)
		}
		err = watchPid(ctx, client, vm, auth, []int64{pid})
		if err != nil {
			log.Error("Setting Address failed", "err", err)
		}
	}

	if netConfig.DNS != "" {
		log.Info("Configuring network", "dns", netConfig.DNS)
		_, err := vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli con mod InfraKit ipv4.dns '%s'", netConfig.DNS), netConfig.SudoUser)
		if err != nil {
			log.Error("Setting DNS failed", "err", err)
		}
	}

	_, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli con mod InfraKit ipv4.method manual"), netConfig.SudoUser)
	if err != nil {
		log.Error("Setting network config failed", "err", err)
	}

	_, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli con mod InfraKit '%s' connection.autoconnect yes", netConfig.DeviceName), netConfig.SudoUser)
	if err != nil {
		log.Error("Setting network config failed", "err", err)
	}

	pid, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli con up InfraKit ifname '%s'", netConfig.DeviceName), netConfig.SudoUser)
	if err != nil {
		log.Error("Setting network config failed", "err", err)
	}
	err = watchPid(ctx, client, vm, auth, []int64{pid})
	if err != nil {
		log.Error("Setting Address failed", "err", err)
	}

	if netConfig.Hostname != "" {
		log.Info("Finalising networking configuration", "hostname", netConfig.Hostname)
		pid, err = vmExec(ctx, client, vm, auth, fmt.Sprintf("nmcli general hostname '%s'", netConfig.Hostname), netConfig.SudoUser)
		if err != nil {
			log.Error("Setting Hostname failed", "err", err)
		}
		err = watchPid(ctx, client, vm, auth, []int64{pid})
		if err != nil {
			log.Error("Setting Address failed", "err", err)
		}
	}
	return nil
}

// RunCommand - This will find a specified VM and run a single command
func (plan *DeploymentPlan) RunCommand(ctx context.Context, client *govmomi.Client, sudoUser string) error {

	f := find.NewFinder(client.Client, true)

	// Find one and only datacenter, not sure how VMware linked mode will work
	dc, err := f.DatacenterOrDefault(ctx, plan.VMWConfig.DCName)
	if err != nil {
		return fmt.Errorf("No Datacenter instance could be found inside of vCenter %v", err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Use finder for VM template
	foundVM, err := f.VirtualMachine(ctx, plan.VMWConfig.VMName)
	if err != nil {
		return err
	}

	auth := &types.NamePasswordAuthentication{
		Username: plan.VMWConfig.VMTemplateAuth.Username,
		Password: plan.VMWConfig.VMTemplateAuth.Password,
	}

	_, err = vmExec(ctx, client, foundVM, auth, plan.VMWConfig.Command, sudoUser)
	if err != nil {
		return err
	}

	return nil
}
