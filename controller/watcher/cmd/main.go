package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
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

func getMap(v interface{}, key string) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		if mm, ok := m[key]; ok {
			if r, ok := mm.(map[string]interface{}); ok {
				return r
			}
		}
	}
	return nil
}

// RestartControllers restart the controllers in the swim file.
// Given the config data, restart any active containers as necessary.
// Initially we assume a simple config that lists the controllers to restart.
// In reality, it's more complicated -- we would parse the swim config and analyze
// to determine the list of controllers that should be running (if new) and which ones
// should be updated.
func RestartControllers(buff []byte) {

	log.Infoln("Change detected. Restarting controllers")

	// Get the controllers
	swim := map[string]interface{}{}
	if err := json.Unmarshal(buff, &swim); err != nil {
		log.Warningln("Error unmarshaling swim:", err, ", No action.")
		return
	}

	images := []string{}

	for resource, block := range swim {
		if driver := getMap(block, "driver"); driver != nil {
			if image, ok := driver["image"]; ok {
				if i, ok := image.(string); ok {
					images = append(images, i)
					log.Infoln("controller image", i, "found for resource", resource)
				}
			}
		}
	}

	for _, image := range images {
		restart := watcher.Restart(docker, image)
		restart.Run()
	}
}
