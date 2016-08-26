package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/client"
	"github.com/docker/libmachete/controller/scaler"
	"github.com/docker/libmachete/controller/util/cli"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

// for hooking up the http server with the scaler object
type backend struct {
	scaler  scaler.Scaler
	config  chan<- []byte
	stop    chan<- struct{}
	done    chan struct{}
	request []byte
}

func main() {
	rootCmd := &cobra.Command{
		Use: "scaler <machete address> <target count> <config path>",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	runCmd := cobra.Command{Use: "run <machete address>"}

	runWhenLeading := cli.LeaderCmd(runCmd)

	driverDir := "/tmp/machete"
	listen := ":9090"

	runCmd.Flags().StringVar(&driverDir, "driver_dir", driverDir, "driver directory")
	runCmd.Flags().StringVar(&listen, "listen", listen, "listen address (unix or tcp)")

	runCmd.Run = func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Usage()
			return
		}

		macheteAddress := args[0]

		// channel from which the configuration will be read.
		start := make(chan []byte)
		running := make(chan scaler.Scaler)
		done := make(chan struct{})

		backend := &backend{
			done:   done,
			config: start,
		}

		log.Infoln("Starting httpd")

		// Now start the http server with proper handling of signals
		stopHTTP, wait, err := runHTTP(driverDir, listen, backend)
		if err != nil {
			panic(err)
		}
		log.Infoln("Started httpd")
		backend.stop = stopHTTP

		shutdown := make(chan struct{}) // for notifying this method that the server has shutdown before activated.

		go func() {
			log.Infoln("Starting scaler core")
		input:
			for {
				log.Infoln("Waiting for config.")
				select {
				case <-wait:
					log.Infoln("httpd stopped. Bye.")
					close(shutdown)
					return

				case requestData := <-start:
					backend.request = requestData
					log.Infoln("Begin running the scaler with configuration:", string(requestData))
					instanceWatcher, err := scaler.NewFixedScaler(
						5*time.Second,
						client.NewInstanceProvisioner(macheteAddress),
						string(requestData))
					if err != nil {
						log.Warningln("Scaler cannot run:", err, "with config=", string(requestData))
						continue input
					}
					running <- instanceWatcher
					log.Infoln("Running the instance watcher.")
					runWhenLeading(instanceWatcher)
					log.Infoln("Instance watcher stopped.")
					close(done)
				}
			}
		}()

		select {
		case s := <-running:
			log.Infoln("instanceWatcher running:", s)
			backend.scaler = s
		case <-shutdown:
			log.Infoln("shutting down. bye.")
		}

		<-wait // for httpd to stop
	}

	rootCmd.AddCommand(&runCmd)

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
