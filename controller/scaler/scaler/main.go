package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/client"
	"github.com/docker/libmachete/controller/scaler"
	"github.com/docker/libmachete/controller/util/cli"
	"github.com/spf13/cobra"
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

	runCmd := cobra.Command{Use: "run <machete address> <target count> <config path>"}

	runWhenLeading := cli.LeaderCmd(runCmd)

	runCmd.Run = func(cmd *cobra.Command, args []string) {
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

		runWhenLeading(instanceWatcher)
	}

	rootCmd.AddCommand(&runCmd)

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
