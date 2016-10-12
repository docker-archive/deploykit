package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/discovery"
	"github.com/docker/infrakit/plugin/group"
	"github.com/docker/infrakit/plugin/util"
	"github.com/docker/infrakit/spi/flavor"
	flavor_client "github.com/docker/infrakit/spi/http/flavor"
	group_server "github.com/docker/infrakit/spi/http/group"
	instance_client "github.com/docker/infrakit/spi/http/instance"
	"github.com/docker/infrakit/spi/instance"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// PluginName is the name of the plugin in the Docker Hub / registry
	PluginName = "GroupPlugin"

	// PluginType is the type / interface it supports
	PluginType = "infra.GroupPlugin/1.0"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {

	logLevel := len(log.AllLevels) - 2
	discoveryDir := "/run/infrakit/plugins/"
	name := "group"

	pollInterval := 10 * time.Second

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Group server",
		RunE: func(c *cobra.Command, args []string) error {

			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			if c.Use == "version" {
				return nil
			}

			discoveryDir = viper.GetString("discovery")
			name = viper.GetString("name")
			listen := fmt.Sprintf("unix://%s/%s.sock", path.Clean(discoveryDir), name)

			// parse the listen string
			listenURL, err := url.Parse(listen)
			if err != nil {
				return err
			}

			log.Infoln("Starting discovery")

			pluginDir, err := discovery.NewDir(filepath.Dir(listenURL.Path))
			if err != nil {
				return err
			}

			instancePluginLookup := func(n string) (instance.Plugin, error) {
				callable, err := pluginDir.PluginByName(n)
				if err != nil {
					return nil, err
				}
				return instance_client.PluginClient(callable), nil
			}

			flavorPluginLookup := func(n string) (flavor.Plugin, error) {
				callable, err := pluginDir.PluginByName(n)
				if err != nil {
					return nil, err
				}
				return flavor_client.PluginClient(callable), nil
			}

			log.Infoln("Starting plugin")

			log.Infoln("Starting")
			log.Infoln("Listening on:", listenURL.String())

			_, stopped, err := util.StartServer(listenURL.String(), group_server.PluginServer(
				group.NewGroupPlugin(
					instancePluginLookup,
					flavorPluginLookup,
					pollInterval)))

			if err != nil {
				log.Error(err)
			}

			<-stopped // block until done

			log.Infoln("Server stopped")
			return nil
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

	cmd.Flags().String("discovery", discoveryDir, "Dir discovery path for plugin discovery")
	// Bind Pflags for cmd passed
	viper.BindEnv("discovery", "INFRAKIT_PLUGINS_DIR")
	viper.BindPFlag("discovery", cmd.Flags().Lookup("discovery"))
	cmd.Flags().String("name", name, "listen socket name for the control endpoint")
	viper.BindPFlag("name", cmd.Flags().Lookup("name"))
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", pollInterval, "Group polling interval")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
