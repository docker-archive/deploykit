package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/plugin/util"
	instance_plugin "github.com/docker/infrakit/spi/http/instance"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// PluginName is the name of the plugin in the Docker Hub / registry
	PluginName = "FileInstance"

	// PluginType is the type / interface it supports
	PluginType = "infrakit.InstancePlugin/1.0"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {

	logLevel := len(log.AllLevels) - 2
	listen := "unix:///run/infrakit/plugins/"
	sock := "instance-file.sock"
	dir := os.TempDir()

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "File instance plugin",
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

			listen = viper.GetString("listen")
			sock = viper.GetString("sock")

			// parse the listen string
			listenURL, err := url.Parse(listen)
			if err != nil {
				return err
			}

			if listenURL.Scheme == "unix" {
				listenURL.Path = path.Join(listenURL.Path, sock)
			}

			log.Infoln("Starting plugin")
			log.Infoln("Listening on:", listenURL.String())

			_, stopped, err := util.StartServer(listenURL.String(), instance_plugin.PluginServer(
				NewFileInstancePlugin(dir)))

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

	cmd.Flags().String("listen", listen, "listen address (unix or tcp) for the control endpoint")
	viper.BindEnv("listen", "INFRAKIT_PLUGINS_LISTEN")
	viper.BindPFlag("listen", cmd.Flags().Lookup("listen"))
	cmd.Flags().String("sock", sock, "listen socket for the control endpoint")
	viper.BindPFlag("sock", cmd.Flags().Lookup("sock"))
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&dir, "dir", dir, "Dir for storing the files")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
