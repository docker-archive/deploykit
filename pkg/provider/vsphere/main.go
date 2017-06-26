package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "VMware vSphere instance plugin",
	}

	// This will hold the configuration that is used to communicate with VMware vCenter or vSphere
	var newVCenter vCenter

	name := cmd.Flags().String("name", "instance-vsphere", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	// Attributes of the VMware vCenter Server to connect to
	newVCenter.vCenterURL = cmd.Flags().String("url", os.Getenv("VCURL"), "URL of VMware vCenter in the format of https://username:password@VCaddress/sdk")
	newVCenter.dsName = cmd.Flags().String("datastore", os.Getenv("VCDATASTORE"), "The name of the DataStore to host the VM")
	newVCenter.networkName = cmd.Flags().String("network", os.Getenv("VCNETWORK"), "The network label the VM will use")
	newVCenter.vSphereHost = cmd.Flags().String("hostname", os.Getenv("VCHOST"), "The server that will run the VM")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, instance_plugin.PluginServer(NewVSphereInstancePlugin(&newVCenter)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
