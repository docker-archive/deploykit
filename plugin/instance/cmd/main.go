package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	plugin "github.com/docker/libmachete/plugin/instance"
	instanceSpi "github.com/docker/libmachete/spi/instance"
	"github.com/spf13/cobra"
	"os"
)

var (
	logLevel = len(log.AllLevels) - 2
	listen   = "/run/docker/plugins/instance.sock"

	// Name is the name of the container image name / plugin name
	Name = "NoName"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

type backend struct {
	plugin instanceSpi.Plugin
}

func info() interface{} {
	return map[string]interface{}{
		"name":       Name,
		"pluginType": "instance.Plugin",
		"version":    Version,
		"revision":   Revision,
		"namespace":  Name,
	}
}

func main() {

	backend := &backend{}

	// Top level main command...  all subcommands are designed to create the watch function
	// for the watcher, except 'version' subcommand.  After the subcommand completes, the
	// post run then begins execution of the actual watcher.
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "Instance plugin for provisioning instances",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {

			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			if c.Use == "version" {
				return nil
			}
			return nil
		},

		// After the subcommand completed we start the main part...
		PersistentPostRunE: func(c *cobra.Command, args []string) error {
			switch c.Use {
			case "version":
				return nil
			default:
			}

			// Subcommands should initialize the backend.plugin
			if backend.plugin == nil {
				return fmt.Errorf("plugin backend not initialized")
			}

			log.Infoln("Starting httpd")
			log.Infoln("Listening on:", listen)

			_, waitHTTP, err := util.StartServer(listen, plugin.NewHandler(backend.plugin, info),
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

	// The subcommand initializes the platform specific provisioner, e.g. aws for AWS, azure for azure
	aws := awsCommand(backend)

	// TODO(chungers) - add azure

	cmd.AddCommand(aws)

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
