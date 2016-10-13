package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/plugin/util"
	instance_plugin "github.com/docker/infrakit/spi/http/instance"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// PluginName is the name of the plugin in the Docker Hub / registry
	PluginName = "TerraformInstance"

	// PluginType is the type / interface it supports
	PluginType = "infrakit.InstancePlugin/1.0"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
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

	logLevel := len(log.AllLevels) - 2
	discoveryDir := "/run/infrakit/plugins/"
	name := "instance-terraform"
	dir := os.TempDir()

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Terraform instance plugin",
		Run: func(c *cobra.Command, args []string) {
			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			discoveryDir = viper.GetString("discovery")
			name = viper.GetString("name")
			listen := fmt.Sprintf("unix://%s/%s.sock", path.Clean(discoveryDir), name)

			_, stopped, err := util.StartServer(listen, instance_plugin.PluginServer(
				NewTerraformInstancePlugin(dir)))

			if err != nil {
				log.Error(err)
			}

			<-stopped // block until done
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			buff, err := json.MarshalIndent(map[string]interface{}{
				"name":     PluginName,
				"type":     PluginType,
				"version":  Version,
				"revision": Revision,
			}, "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(buff))
			return nil
		},
	})

	cmd.Flags().String("discovery", discoveryDir, "Dir discovery path for plugin discovery")
	// Bind Pflags for cmd passed
	viper.BindEnv("discovery", "INFRAKIT_PLUGINS_DIR")
	viper.BindPFlag("discovery", cmd.Flags().Lookup("discovery"))
	cmd.Flags().String("name", name, "Plugin name to advertise for the control endpoint")
	viper.BindPFlag("name", cmd.Flags().Lookup("name"))
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&dir, "dir", dir, "Dir for storing the files")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
