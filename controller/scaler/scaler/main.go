package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/client"
	"github.com/docker/libmachete/controller/scaler"
	"github.com/docker/libmachete/spi/instance"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "scaler <machete address> <instance group> <target count> <config path>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 4 {
				cmd.Usage()
				return
			}

			macheteAddress := args[0]
			group := args[1]

			targetCount, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				log.Error("Invalid target count", err)
				os.Exit(1)
			}

			configPath := args[3]

			requestData, err := ioutil.ReadFile(configPath)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			instanceWatcher := scaler.NewFixedScaler(
				5*time.Second,
				client.NewInstanceProvisioner(macheteAddress),
				string(requestData),
				instance.GroupID(group),
				uint(targetCount))

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				for range c {
					log.Info("Stopping scaler")
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
