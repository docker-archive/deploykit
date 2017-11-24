package x

import (
	"context"
	"io/ioutil"
	"os"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/x/vmwscript"
	"github.com/spf13/cobra"
)

var cmdResults = map[string]string{}

//var log = logutil.New("module", "x/vmwscript") /
var debugV = logutil.V(200) // 100-500 are for typical debug levels, > 500 for highly repetitive logs (e.g. from polling)

func vmwscriptCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "vmwscript deployment.json",
		Short: "This tool uses the native VMware APIs to automate Virtual Machines through the guest tools",
	}

	plan := vmwscript.DeploymentPlan{}
	cmd.Flags().StringVar(&plan.VMWConfig.VCenterURL, "vcurl", os.Getenv("INFRAKIT_VSPHERE_VCURL"),
		"VMware vCenter URL, format https://user:pass@address/sdk [REQD]")
	cmd.Flags().StringVar(&plan.VMWConfig.DCName, "datacenter", os.Getenv("INFRAKIT_VSPHERE_VCDATACENTER"),
		"The name of the Datacenter to host the VM [REQD]")
	cmd.Flags().StringVar(&plan.VMWConfig.DSName, "datastore", os.Getenv("INFRAKIT_VSPHERE_VCDATASTORE"),
		"The name of the DataStore to host the VM [REQD]")
	cmd.Flags().StringVar(&plan.VMWConfig.NetworkName, "network", os.Getenv("INFRAKIT_VSPHERE_VCNETWORK"),
		"The network label the VM will use [REQD]")
	cmd.Flags().StringVar(&plan.VMWConfig.VSphereHost, "hostname", os.Getenv("INFRAKIT_VSPHERE_VCHOST"),
		"The server that will run the VM [REQD]")
	cmd.Flags().StringVar(&plan.VMWConfig.VMTemplateAuth.Username, "templateUser", os.Getenv("INFRAKIT_VSPHERE_VMUSER"),
		"A created user inside of the VM template")
	cmd.Flags().StringVar(&plan.VMWConfig.VMTemplateAuth.Password, "templatePass", os.Getenv("INFRAKIT_VSPHERE_VMPASS"),
		"The password for the specified user inside the VM template")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Check that the argument (the json file exists)
		if len(args) == 0 {
			cmd.Usage()
			log.Crit("Please specify the path to a configuration file")
			os.Exit(-1)
		}

		// Attempt to open file
		buff, err := ioutil.ReadFile(args[0])
		if err != nil {
			log.Crit("Error opening file", "Error", err)
			return err
		}

		err = types.AnyBytes(buff).Decode(&plan)
		if err != nil {
			log.Crit("Error parsing file", "Error", err)
			return err
		}

		err = plan.Validate()
		if err != nil {
			log.Crit("Error validating input", "Error", err)
			os.Exit(-1)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client, err := vmwscript.VCenterLogin(ctx, plan.VMWConfig)
		if err != nil {
			log.Crit("Error connecting to vCenter", "err", err)
			os.Exit(-1)
		}
		// Iterate through the deployments and tasks
		log.Info("Starting VMwScript engine")
		plan.RunTasks(ctx, client)
		log.Info("VMwScript has completed succesfully")
		return nil
	}

	return cmd
}
