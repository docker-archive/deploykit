package main

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/plugin/flavor/swarm"
	"github.com/docker/infrakit/plugin/util"
	flavor_plugin "github.com/docker/infrakit/spi/http/flavor"
	"github.com/spf13/cobra"
)

var (
	// PluginName is the name of the plugin in the Docker Hub / registry
	PluginName = "SwarmFlavor"

	// PluginType is the type / interface it supports
	PluginType = "infrakit.FlavorPlugin/1.0"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {

	logLevel := len(log.AllLevels) - 2
	listen := "unix:///run/infrakit/plugins/flavor-swarm.sock"

	tlsOptions := tlsconfig.Options{}
	host := "unix:///var/run/docker.sock"

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Docker Swarm flavor plugin",
		Run: func(c *cobra.Command, args []string) {

			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			dockerClient, err := NewDockerClient(host, &tlsOptions)
			log.Infoln("Connect to docker", host, "err=", err)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			_, stopped, err := util.StartServer(listen, flavor_plugin.PluginServer(swarm.NewSwarmFlavor(dockerClient)))

			if err != nil {
				log.Error(err)
			}

			<-stopped // block until done
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			buff, err := json.MarshalIndent(map[string]interface{}{
				"name":     PluginName,
				"type":     PluginType,
				"version":  Version,
				"revision": Revision,
			}, "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(buff))
			return nil
		},
	})

	cmd.PersistentFlags().StringVar(&listen, "listen", listen, "listen address (unix or tcp) for the control endpoint")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.PersistentFlags().StringVar(&host, "host", host, "Docker host")
	cmd.PersistentFlags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert file path")
	cmd.PersistentFlags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert file path")
	cmd.PersistentFlags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key file path")
	cmd.PersistentFlags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
