package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	instance "github.com/docker/infrakit/pkg/plugin/instance/libvirt"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_plugin "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/run"
	instance_spi "github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Libvirt instance plugin",
	}

	name := cmd.Flags().String("name", "instance-libvirt", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	uri := cmd.Flags().String("uri", "qemu:///session", "libvirt URI to connect to")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		cli.SetLogLevel(*logLevel)
		run.Plugin(plugin.DefaultTransport(*name),
			instance_plugin.PluginServer(instance.NewLibvirtPlugin(*uri)),
			metadata_plugin.PluginServer(metadata.NewPluginFromData(
				map[string]interface{}{
					"version":    cli.Version,
					"revision":   cli.Revision,
					"implements": instance_spi.InterfaceSpec,
				},
			)),
		)
		return nil
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			buff, err := json.MarshalIndent(map[string]interface{}{
				"version":  cli.Version,
				"revision": cli.Revision,
			}, "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(buff))
			return nil
		},
	})

	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func getHome() string {
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return os.Getenv("HOME")
}
