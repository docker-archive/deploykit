package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/client"
	"github.com/docker/libmachete/controller/quorum"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "quorum <machete address> <quorum IP addresses> <config path>",
		Long: `
quorum manages a group of instances that require a fixed set of instances that
require fixed IP addresses.

Any nodes provisioned will be allocated one of the configured IP addresses, and
the controller will converge towards having every IP address represented.

The configuration file provided must be a JSON-formatted instance provisioning
request (suitable for use with the driver running at the target machete server).
The provisioning request must be a go-style template, with '{{.IP}}' included.
This parameter will be substituted with one of the provided quorum IP addresses
when a quorum member is absent.`,

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 3 {
				cmd.Usage()
				return
			}

			macheteAddress := args[0]
			ipAddresses := strings.Split(args[1], ",")
			configPath := args[2]

			requestData, err := ioutil.ReadFile(configPath)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			instanceWatcher, err := quorum.NewQuorum(
				5*time.Second,
				client.NewInstanceProvisioner(macheteAddress),
				string(requestData),
				ipAddresses)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				for range c {
					log.Info("Stopping quorum")
					instanceWatcher.Stop()
				}
			}()

			instanceWatcher.Run()

			if err != nil {
				log.Error(err)
				os.Exit(1)
			}
		},
	}

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
