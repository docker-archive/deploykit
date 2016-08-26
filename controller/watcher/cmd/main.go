package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/libmachete/controller"
	"github.com/docker/libmachete/controller/watcher"
	"github.com/spf13/cobra"
)

const (
	// Default host value borrowed from github.com/docker/docker/opts
	defaultHost = "unix:///var/run/docker.sock"
)

var (
	tlsOptions = tlsconfig.Options{}
	logLevel   = len(log.AllLevels) - 2
	host       = defaultHost
	driverDir  = "/tmp/machete"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func dockerClient() client.APIClient {
	client, err := watcher.NewDockerClient(host, &tlsOptions)
	if err != nil {
		panic(err)
	}
	return client
}

// Singleton initialzed in the start up
var docker client.APIClient

// The controller registry.  For discovering how to connect to what drivers.
var registry *controller.Registry

func main() {

	cmd := &cobra.Command{
		Use:   "watcher",
		Short: "Watches for change of some resource and performs some action.",
		PersistentPreRunE: func(_ *cobra.Command, args []string) error {
			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			// Populate the registry of drivers
			r, err := controller.NewRegistry(driverDir)
			if err != nil {
				return err
			}

			// Sets up global singleton
			registry = r
			docker = dockerClient()
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	var host string

	cmd.PersistentFlags().StringVar(&driverDir, "driver_dir", driverDir, "Directory for driver/plugin discovery")
	cmd.PersistentFlags().StringVar(&host, "host", defaultHost, "Docker host")
	cmd.PersistentFlags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key")
	cmd.PersistentFlags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.AddCommand(watchURL())

	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
