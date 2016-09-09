package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete.aws"
	plugin "github.com/docker/libmachete.aws/plugin/instance"
	"github.com/docker/libmachete/controller/util"
	"github.com/spf13/cobra"
	"os"
)

var (
	logLevel = len(log.AllLevels) - 2
	listen   = "/run/docker/plugins/aws-instance.sock"
	// PluginName is the name of the plugin in the Docker Hub / registry
	PluginName = "NoPluginName"

	// PluginType is the name of the container image name / plugin name
	PluginType = "docker.instanceDriver/1.0"

	// PluginNamespace is the namespace of the plugin
	PluginNamespace = "/aws/instance"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func info() interface{} {
	return map[string]interface{}{
		"name":      PluginName,
		"type":      PluginType,
		"namespace": PluginNamespace,
		"version":   Version,
		"revision":  Revision,
	}
}

func main() {

	builder := &aws.Builder{}

	// Top level main command...  all subcommands are designed to create the watch function
	// for the watcher, except 'version' subcommand.  After the subcommand completes, the
	// post run then begins execution of the actual watcher.
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "Instance plugin for provisioning instances",
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
			provisioner, err := builder.BuildInstancePlugin()
			if err != nil {
				log.Error(err)
				return err
			}

			log.Infoln("Starting httpd")
			log.Infoln("Listening on:", listen)

			_, waitHTTP, err := util.StartServer(listen, plugin.NewHandler(provisioner, info),
				func() error {
					log.Infoln("Shutting down.")
					return nil
				})
			if err != nil {
				panic(err)
			}
			log.Infoln("Started httpd")

			<-waitHTTP
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			buff, err := json.MarshalIndent(info(), "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(buff))
			return nil
		},
	})

	cmd.PersistentFlags().StringVar(&listen, "listen", listen, "listen address (unix or tcp)")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	// TODO(chungers) - the exposed flags here won't be set in plugins, because plugin install doesn't allow
	// user to pass in command line args like containers with entrypoint.
	cmd.Flags().AddFlagSet(builder.Flags())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
