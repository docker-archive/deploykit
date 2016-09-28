package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/discovery"
	"github.com/spf13/cobra"
)

// This is a generic client for infrakit

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {

	logLevel := len(log.AllLevels) - 2
	discoveryDir := "/run/infrakit/plugins"

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "infrakit cli",
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
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			buff, err := json.MarshalIndent(map[string]interface{}{
				"description": "infrakit cli",
				"version":     Version,
				"revision":    Revision,
			}, "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(buff))
			return nil
		},
	})

	f := func() *discovery.Dir {
		d, err := discovery.NewDir(discoveryDir)
		if err != nil {
			log.Errorf("Failed to initialize plugin discovery: %s", err)
			os.Exit(1)
		}
		return d
	}
	cmd.AddCommand(pluginCommand(f), instancePluginCommand(f), groupPluginCommand(f), flavorPluginCommand(f))

	cmd.PersistentFlags().StringVar(&discoveryDir, "dir", discoveryDir, "Dir path for plugin discovery")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func assertNotNil(message string, f interface{}) {
	if f == nil {
		log.Error(errors.New(message))
		os.Exit(1)
	}
}

func getInput(args []string) []byte {
	input := os.Stdin
	if len(args) > 0 {
		i, err := os.Open(args[0])
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		input = i
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	return buff
}
