package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
	"github.com/docker/libmachete/controller"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// docker run -v /var/run/:/var/run -v /run/docker:/run/docker chungers/hello client chungers/hello-plugin info
func clientCommand(backend *backend) *cobra.Command {

	client := &cobra.Command{
		Use:   "client",
		Short: "Runs the client",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("missing plugin name and op")
			}
			pluginName := args[0]
			log.Infoln("Looking for plugin", pluginName)

			if backend.docker == nil {
				return fmt.Errorf("err-no-docker")
			}

			// discover the plugin
			plugins, err := backend.docker.PluginList(context.Background())
			if err != nil {
				return err
			}

			var found *types.Plugin
			for _, plugin := range plugins {
				if plugin.Name == pluginName {
					found = plugin
					break
				}
			}

			if found == nil {
				return fmt.Errorf("plugin not found: %s", pluginName)
			}

			if !found.Active {
				return fmt.Errorf("plugin not active: %s, id=%s", found.Name, found.ID)
			}

			pluginSocket := fmt.Sprintf("/run/docker/%s/%s", found.ID, found.Manifest.Interface.Socket)

			log.Infoln("For plugin", found.Name, "socket=", pluginSocket)

			// now connect -- this assumes the volume is bind mounted...
			client := controller.NewClient(pluginSocket)

			op := args[1]
			var req interface{}
			if len(args) == 3 {
				a := map[string]interface{}{}
				if err := json.Unmarshal([]byte(args[2]), &a); err == nil {
					req = a
				} else {
					return err
				}
			}

			switch op {
			case "info":
				info, err := client.Info()
				if err != nil {
					return err
				}
				log.Infoln("Info:", info)
			default:
				return client.Call(op, req)
			}
			return nil
		},
	}
	return client
}
