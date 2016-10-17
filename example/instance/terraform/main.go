package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	instance_plugin "github.com/docker/infrakit/spi/http/instance"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

func mustHaveTerraform() {
	// check if terraform exists
	if _, err := exec.LookPath("terraform"); err != nil {
		log.Error("Cannot find terraform.  Please install at https://www.terraform.io/downloads.html")
		os.Exit(1)
	}
}

func main() {

	mustHaveTerraform()

	logLevel := cli.DefaultLogLevel
	name := "instance-terraform"
	dir := os.TempDir()

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Terraform instance plugin",
		Run: func(c *cobra.Command, args []string) {
			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, instance_plugin.PluginServer(NewTerraformInstancePlugin(dir)))
		},
	}

	cmd.AddCommand(cli.VersionCommand())

	cmd.Flags().String("name", name, "Plugin name to advertise for discovery")
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&dir, "dir", dir, "Dir for storing plan files")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
