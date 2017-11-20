package x

import (
	"context"
	"os"

	logutil "github.com/docker/infrakit/pkg/log"

	"github.com/docker/infrakit/pkg/x/vmwscript"
	"github.com/spf13/cobra"
)

var cmdResults = map[string]string{}

//var log = logutil.New("module", "x/vmwscript") /
var debugV = logutil.V(200) // 100-500 are for typical debug levels, > 500 for highly repetitive logs (e.g. from polling)

func vmwscriptCommand() *cobra.Command {
	vmwscript.InitDeployment()
	vm := vmwscript.VMwareConfig() //Pull VMware configuration from JSON

	var vc, dc, ds, nn, vh, gu, gp *string

	cmd := &cobra.Command{
		Use:   "vmwscript deployment.json",
		Short: "This tool uses the native VMware APIs to automate Virtual Machines through the guest tools",
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Check that the argument (the json file exists)
		if len(args) == 0 {
			cmd.Usage()
			log.Crit("Please specify the path to a configuration file")
			os.Exit(-1)
		}
		err := vmwscript.OpenFile(args[0])
		if err != nil {
			log.Crit("Error opening file", "Error", err)
			os.Exit(-1)
		}

		// if configuration isn't set in JSON, check Environment vars/flags
		if (vm.VCenterURL == nil) || *vm.VCenterURL == "" {
			if vm.VCenterURL = vc; *vm.VCenterURL == "" {
				log.Crit("VMware vCenter/vSphere credentials are missing")
				os.Exit(-1)
			}
		}

		if (vm.DCName == nil) || *vm.DCName == "" {
			if vm.DCName = dc; *vm.DCName == "" {
				log.Error("No Datacenter was specified, will try to use the default (will cause errors with Linked-Mode)")
			}
		}

		if (vm.DSName == nil) || *vm.DSName == "" {
			if vm.DSName = ds; *vm.DSName == "" {
				log.Crit("A VMware vCenter datastore is required for provisioning")
				os.Exit(-1)
			}
		}

		if (vm.NetworkName == nil) || *vm.NetworkName == "" {
			if vm.NetworkName = nn; *vm.NetworkName == "" {
				log.Crit("Specify a Network to connect to")
				os.Exit(-1)
			}
		}

		if (vm.VSphereHost == nil) || *vm.VSphereHost == "" {
			if vm.VSphereHost = vh; *vm.VSphereHost == "" {
				log.Crit("A Host inside of vCenter/vSphere is required to provision on for VM capacity")
				os.Exit(-1)
			}
		}

		// Ideally these should be populated as they're needed for a lot of the tasks.
		if (vm.VMTemplateAuth.Username == nil) || *vm.VMTemplateAuth.Username == "" {
			if vm.VMTemplateAuth.Username = gu; *vm.VMTemplateAuth.Username == "" {
				log.Error("No Username for inside of the Guest OS was specified, somethings may fail")
			}
		}

		if (vm.VMTemplateAuth.Password == nil) || *vm.VMTemplateAuth.Password == "" {
			if vm.VMTemplateAuth.Password = gp; *vm.VMTemplateAuth.Username == "" {
				log.Error("No Password for inside of the Guest OS was specified, somethings may fail")
			}
		}

		if *vm.VCenterURL == "" || *vm.DSName == "" || *vm.VSphereHost == "" || len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client, err := vmwscript.VCenterLogin(ctx, *vm)
		if err != nil {
			log.Crit("Error connecting to vCenter", "err", err)
			os.Exit(-1)
		}
		// Iterate through the deployments and tasks
		log.Info("Starting VMwScript engine")
		vmwscript.RunTasks(ctx, client)
		log.Info("VMwScript has completed succesfully")
		return nil
	}

	vc = cmd.Flags().String("vcurl", os.Getenv("INFRAKIT_VSPHERE_VCURL"), "VMware vCenter URL, format https://user:pass@address/sdk [REQD]")
	dc = cmd.Flags().String("datacenter", os.Getenv("INFRAKIT_VSPHERE_VCDATACENTER"), "The name of the Datacenter to host the VM [REQD]")
	ds = cmd.Flags().String("datastore", os.Getenv("INFRAKIT_VSPHERE_VCDATASTORE"), "The name of the DataStore to host the VM [REQD]")
	nn = cmd.Flags().String("network", os.Getenv("INFRAKIT_VSPHERE_VCNETWORK"), "The network label the VM will use [REQD]")
	vh = cmd.Flags().String("hostname", os.Getenv("INFRAKIT_VSPHERE_VCHOST"), "The server that will run the VM [REQD]")
	gu = cmd.Flags().String("templateUser", os.Getenv("INFRAKIT_VSPHERE_VMUSER"), "A created user inside of the VM template")
	gp = cmd.Flags().String("templatePass", os.Getenv("INFRAKIT_VSPHERE_VMPASS"), "The password for the specified user inside the VM template")
	return cmd
}
