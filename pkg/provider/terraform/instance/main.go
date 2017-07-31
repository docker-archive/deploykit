package main

import (
	"os"
	"os/exec"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func mustHaveTerraform() {
	// check if terraform exists
	if _, err := exec.LookPath("terraform"); err != nil {
		log.Error("Cannot find terraform.  Please install at https://www.terraform.io/downloads.html")
		os.Exit(1)
	}
}

func getDir() string {
	dir := os.Getenv("INFRAKIT_INSTANCE_TERRAFORM_DIR")
	if dir != "" {
		return dir
	}
	return os.TempDir()
}

type bootstrapOptions struct {
	InstanceID   *string
	GroupSpecURL *string
}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Terraform instance plugin",
	}
	name := cmd.Flags().String("name", "instance-terraform", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	dir := cmd.Flags().String("dir", getDir(), "Dir for storing plan files")
	pollInterval := cmd.Flags().Duration("poll-interval", 30*time.Second, "Terraform polling interval")
	standalone := cmd.Flags().Bool("standalone", false, "Set if running standalone, disables manager leadership verification")
	// Bootstrap options
	bootstrapGrpSpec := cmd.Flags().String("bootstrap-group-spec-url", "", "Group spec to import the current into into, must be used with 'bootstrap-instance-id'")
	bootstrapInstID := cmd.Flags().String("bootstrap-instance-id", "", "Current instance ID, must be used with 'bootstrap-group-spec-url'")
	bootstrap := bootstrapOptions{
		InstanceID:   bootstrapInstID,
		GroupSpecURL: bootstrapGrpSpec,
	}
	cmd.Run = func(c *cobra.Command, args []string) {
		mustHaveTerraform()
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, instance_plugin.PluginServer(
			NewTerraformInstancePlugin(*dir, *pollInterval, *standalone, &bootstrap)),
		)
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
