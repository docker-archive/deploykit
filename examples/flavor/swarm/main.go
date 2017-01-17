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
	initScriptTemplURL := cmd.Flags().String("init-template", "", "Init script template file, in URL form")

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

		var templ *template.Template
		if *initScriptTemplURL == "" {
			t, err := template.NewTemplate("str://"+DefaultInitScriptTemplate, opts)
			if err != nil {
				return err
			}
			templ = t
		} else {

			t, err := template.NewTemplate(*initScriptTemplURL, opts)
			if err != nil {
				return err
			}
			templ = t
		}

		cli.RunPlugin(*name, flavor_plugin.PluginServerWithTypes(
			map[string]flavor.Plugin{
				"manager": NewManagerFlavor(dockerClient, templ),
				"worker":  NewWorkerFlavor(dockerClient, templ),
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

const (
	// DefaultInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm.
	DefaultInitScriptTemplate = `
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

mkdir -p /etc/docker
cat << EOF > /etc/docker/daemon.json
{
  "labels": ["swarm-association-id={{.ASSOCIATION_ID}}"]
}
EOF

{{.RESTART_DOCKER}}

docker swarm join {{.MY_IP}} --token {{.JOIN_TOKEN}}
`
)
