package main

import (
	"errors"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/spf13/cobra"
)

// A generic client for infrakit

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "infrakit cli",
	}
	logLevel := cmd.PersistentFlags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.PersistentPreRun = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)
	}

	// Don't print usage text for any error returned from a RunE function.  Only print it when explicitly requested.
	cmd.SilenceUsage = true

	// Don't automatically print errors returned from a RunE function.  They are returned from cmd.Execute() below
	// and we print it ourselves.
	cmd.SilenceErrors = true
	f := func() discovery.Plugins {
		d, err := discovery.NewPluginDiscovery()
		if err != nil {
			log.Fatalf("Failed to initialize plugin discovery: %s", err)
			os.Exit(1)
		}
		return d
	}

	cmd.AddCommand(cli.VersionCommand(), cli.InfoCommand(f))
	cmd.AddCommand(pluginCommand(f), instancePluginCommand(f), groupPluginCommand(f), flavorPluginCommand(f))

	err := cmd.Execute()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func assertNotNil(message string, f interface{}) {
	if f == nil {
		log.Error(errors.New(message))
		os.Exit(1)
	}
}
