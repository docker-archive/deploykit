package main

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.aws/plugin/instance"
	"github.com/docker/infrakit/plugin/util"
	instance_plugin "github.com/docker/infrakit/spi/http/instance"
	"github.com/spf13/cobra"
)

var (
	// PluginName is the name of the plugin in the Docker Hub / registry
	PluginName = "VagrantInstance"

	// PluginType is the type / interface it supports
	PluginType = "infrakit.InstancePlugin/1.0"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {

	builder := &instance.Builder{}
	logLevel := len(log.AllLevels) - 2
	listen := "unix:///run/infrakit/plugins/instance-vagrant.sock"

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "AWS instance plugin",
		RunE: func(c *cobra.Command, args []string) error {

			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			if c.Use == "version" {
				return nil
			}

			instancePlugin, err := builder.BuildInstancePlugin()
			if err != nil {
				log.Error(err)
				return err
			}

			log.Infoln("Starting plugin")
			log.Infoln("Listening on:", listen)

			_, stopped, err := util.StartServer(listen, instance_plugin.PluginServer(instancePlugin))

			if err != nil {
				log.Error(err)
			}

			<-stopped // block until done

			log.Infoln("Server stopped")
			return nil
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

	cmd.Flags().StringVar(&listen, "listen", listen, "listen address (unix or tcp) for the control endpoint")
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	// TODO(chungers) - the exposed flags here won't be set in plugins, because plugin install doesn't allow
	// user to pass in command line args like containers with entrypoint.
	cmd.Flags().AddFlagSet(builder.Flags())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
