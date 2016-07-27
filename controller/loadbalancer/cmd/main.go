package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/cobra"
)

var (
	// Default host value borrowed from github.com/docker/docker/opts
	host       = fmt.Sprintf("unix://%s", "/var/run/docker.sock")
	tlsOptions = tlsconfig.Options{}
	logLevel   = len(log.AllLevels) - 1
)

func main() {
	cmd := &cobra.Command{
		Use:   "loadbalancer",
		Short: "Load Balancer Controller for Editions",
		PersistentPreRunE: func(_ *cobra.Command, args []string) error {
			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&host, "host", host, "Docker host")
	cmd.PersistentFlags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key")
	cmd.PersistentFlags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.AddCommand(runCommand(), elbCommand(), albCommand(), dockerCommand())

	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
