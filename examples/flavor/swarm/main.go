package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/spi/flavor"
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

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Docker Swarm flavor plugin",
	}
	name := cmd.Flags().String("name", "flavor-swarm", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	host := cmd.Flags().String("host", "unix:///var/run/docker.sock", "Docker host")
	caFile := cmd.Flags().String("tlscacert", "", "TLS CA cert file path")
	certFile := cmd.Flags().String("tlscert", "", "TLS cert file path")
	tlsKey := cmd.Flags().String("tlskey", "", "TLS key file path")
	insecureSkipVerify := cmd.Flags().Bool("tlsverify", true, "True to skip TLS")
	managerInitScriptTemplURL := cmd.Flags().String("manager-init-template", "", "URL, init script template for managers")
	workerInitScriptTemplURL := cmd.Flags().String("worker-init-template", "", "URL, init script template for workers")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)

		dockerClient, err := docker.NewDockerClient(*host, &tlsconfig.Options{
			CAFile:             *caFile,
			CertFile:           *certFile,
			KeyFile:            *tlsKey,
			InsecureSkipVerify: *insecureSkipVerify,
		})
		log.Infoln("Connect to docker", host, "err=", err)
		if err != nil {
			return err
		}

		opts := template.Options{
			SocketDir: discovery.Dir(),
		}

		mt, err := getTemplate(*managerInitScriptTemplURL, DefaultManagerInitScriptTemplate, opts)
		if err != nil {
			return err
		}
		wt, err := getTemplate(*workerInitScriptTemplURL, DefaultWorkerInitScriptTemplate, opts)
		if err != nil {
			return err
		}

		cli.RunPlugin(*name, flavor_plugin.PluginServerWithTypes(
			map[string]flavor.Plugin{
				"manager": NewManagerFlavor(dockerClient, mt),
				"worker":  NewWorkerFlavor(dockerClient, wt),
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
