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
	listen     = ":9091"

	// Running state is used to tell the watcher if it's in a special state of running
	// when it comes up the first time in the vpc -- so it will restart the controllers
	// with the first config it detected.
	state = ""

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

type backend struct {
	docker      client.APIClient
	registry    *controller.Registry
	watcher     *watcher.Watcher
	watcherDone <-chan struct{}
	data        chan<- []byte
}

func main() {

	backend := &backend{}

	// Top level main command...  all subcommands are designed to create the watch function
	// for the watcher, except 'version' subcommand.  After the subcommand completes, the
	// post run then begins execution of the actual watcher.

	cmd := &cobra.Command{
		Use:   "watcher",
		Short: "Watches for change of some resource and performs some action.",
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

			// Populate the registry of drivers
			r, err := controller.NewRegistry(driverDir)
			if err != nil {
				return err
			}
			backend.registry = r

			// Docker client
			d, err := watcher.NewDockerClient(host, &tlsOptions)
			if err != nil {
				return err
			}

			backend.docker = d

			// watcher -- the watch function will be set by the user's choice of subcommands (except version)
			backend.watcher = watcher.New(nil, backend.POC2Reactor)

			// make a new channel to send data in, in addition to the one allocated for the watch function
			inbound := make(chan []byte)
			backend.data = inbound
			backend.watcher.AddInbound(inbound)

			return nil
		},

		// After the subcommand completed we start the main part...
		PersistentPostRunE: func(c *cobra.Command, args []string) error {
			if c.Use == "version" {
				return nil
			}

			log.Infoln("Starting httpd")
			_, waitHTTP, err := runHTTP(driverDir, listen, backend)
			if err != nil {
				panic(err)
			}
			log.Infoln("Started httpd")

			log.Infoln("Starting watcher")
			if state == "running" {
				backend.watcher.ReactOnStart(true)
			}
			w, err := backend.watcher.Run()
			if err != nil {
				return err
			}
			log.Infoln("Started watcher")

			backend.watcherDone = w

			<-waitHTTP
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

	cmd.PersistentFlags().StringVar(&driverDir, "driver_dir", driverDir, "Directory for driver/plugin discovery")
	cmd.PersistentFlags().StringVar(&listen, "listen", listen, "listen address (unix or tcp)")
	cmd.PersistentFlags().StringVar(&host, "host", defaultHost, "Docker host")
	cmd.PersistentFlags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key")
	cmd.PersistentFlags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.PersistentFlags().StringVar(&state, "state", state, "set to 'running' will immediately update all controllers on restart")

	// The subcommand is run only to set up the data source.
	cmd.AddCommand(watchURL(backend))

	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
