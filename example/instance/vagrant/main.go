package main

import (
	"os"
<<<<<<< HEAD
	"text/template"
=======
>>>>>>> ba0155815ea4622affab23ce6558ba53e45e62a0

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	"github.com/docker/infrakit/plugin/instance/vagrant"
	instance_plugin "github.com/docker/infrakit/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {

	var name string
	var logLevel int
	var dir string
	var templFile string

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Vagrant instance plugin",
		Run: func(c *cobra.Command, args []string) {
			templ := template.Must(template.New("").Parse(vagrant.VagrantFile))
			if _, err := os.Stat(templFile); err == nil {
				templ = template.Must(template.ParseFiles(templFile))
			}

			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, instance_plugin.PluginServer(vagrant.NewVagrantPlugin(dir, templ)))
		},
	}

	cmd.AddCommand(cli.VersionCommand())

	cmd.Flags().StringVar(&name, "name", "instance-vagrant", "Plugin name to advertise for discovery")
	cmd.PersistentFlags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	defaultDir, err := os.Getwd()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	cmd.Flags().StringVar(&dir, "dir", defaultDir, "Vagrant directory")
	cmd.Flags().StringVar(&templFile, "template", templFile, "Vagrant Template file")

	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
