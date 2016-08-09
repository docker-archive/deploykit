package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	docker_client "github.com/docker/engine-api/client"
	"github.com/docker/libmachete/client"
	"github.com/docker/libmachete/controller/scaler"
	"github.com/docker/libmachete/controller/util/swarm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"time"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

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

	var dockerSocket string
	var pollInterval time.Duration
	runCmd := cobra.Command{
		Use: "run <machete address> <target count> <config path>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 3 {
				cmd.Usage()
				return
			}

			macheteAddress := args[0]

			targetCount, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				log.Error("Invalid target count", err)
				os.Exit(1)
			}

			configPath := args[2]

			requestData, err := ioutil.ReadFile(configPath)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			instanceWatcher, err := scaler.NewFixedScaler(
				5*time.Second,
				client.NewInstanceProvisioner(macheteAddress),
				string(requestData),
				uint(targetCount))
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				for range c {
					log.Info("Stopping scaler")
					instanceWatcher.Stop()
				}
			}()

			dockerClient, err := docker_client.NewClient(
				fmt.Sprintf("unix://%s", dockerSocket),
				"v1.24",
				nil,
				map[string]string{})
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			err = swarm.RunWhenLeading(
				context.Background(),
				dockerClient,
				pollInterval,
				func() {
					go instanceWatcher.Run()
				},
				func() {
					instanceWatcher.Stop()
				})
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			if err != nil {
				log.Error(err)
				os.Exit(1)
			}
		},
	}
	runCmd.Flags().StringVar(&dockerSocket, "docker-socket", "/var/run/docker.sock", "Docker daemon socket path")
	runCmd.Flags().DurationVar(
		&pollInterval,
		"poll-interval",
		5*time.Second,
		"How often to poll for local Docker Engine leadership status")

	rootCmd.AddCommand(&runCmd)

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
