package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	metadata_plugin "github.com/docker/infrakit/pkg/rpc/metadata"
	flavor_spi "github.com/docker/infrakit/pkg/spi/flavor"
	metadata_spi "github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/spf13/cobra"
)

func init() {
	cli.RegisterInfo("swarm-flavor",
		map[string]interface{}{
			"DockerClientAPIVersion": docker.ClientVersion,
		})
}

var defaultTemplateOptions = template.Options{
	SocketDir: discovery.Dir(),
}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Docker Swarm flavor plugin",
	}
	name := cmd.Flags().String("name", "flavor-swarm", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	managerInitScriptTemplURL := cmd.Flags().String("manager-init-template", "", "URL, init script template for managers")
	workerInitScriptTemplURL := cmd.Flags().String("worker-init-template", "", "URL, init script template for workers")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)

		mt, err := getTemplate(*managerInitScriptTemplURL, DefaultManagerInitScriptTemplate, defaultTemplateOptions)
		if err != nil {
			return err
		}
		wt, err := getTemplate(*workerInitScriptTemplURL, DefaultWorkerInitScriptTemplate, defaultTemplateOptions)
		if err != nil {
			return err
		}

		cli.RunPlugin(*name,
			metadata_plugin.PluginServerWithTypes(
				map[string]metadata_spi.Plugin{
					"manager": metadata.NewPluginFromData(map[string]interface{}{
						"version": "1.0",
					}),
					"worker": metadata.NewPluginFromData(map[string]interface{}{
						"version": "2.0",
					}),
				}),
			flavor_plugin.PluginServerWithTypes(
				map[string]flavor_spi.Plugin{
					"manager": NewManagerFlavor(DockerClient, mt),
					"worker":  NewWorkerFlavor(DockerClient, wt),
				}))
		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func getTemplate(url string, defaultTemplate string, opts template.Options) (t *template.Template, err error) {
	if url == "" {
		t, err = template.NewTemplate("str://"+defaultTemplate, opts)
		return
	}
	t, err = template.NewTemplate(url, opts)
	return
}
