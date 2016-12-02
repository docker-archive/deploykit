package main

import (
	"os"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Vagrant instance plugin",
	}
	name := cmd.Flags().String("name", "instance-vagrant", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	defaultDir, err := os.Getwd()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	dir := cmd.Flags().String("dir", defaultDir, "Vagrant directory")
	templFile := cmd.Flags().String("template", "", "Vagrant Template file")
	cmd.RunE = func(c *cobra.Command, args []string) error {

		var templ *template.Template
		if *templFile == "" {
			templ = template.Must(template.New("").Parse(VagrantFile))
		} else {
			var err error
			templ, err = template.ParseFiles()
			if err != nil {
				return err
			}
		}

		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, instance_plugin.PluginServer(NewVagrantPlugin(*dir, templ)))
		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

// VagrantFile is the minimum definition of the vagrant file
const VagrantFile = `
Vagrant.configure("2") do |config|
  config.vm.box = "{{.Properties.Box}}"
  config.vm.hostname = "infrakit.box"
  config.vm.network "private_network"{{.NetworkOptions}}
  config.vm.provision :shell, path: "boot.sh"
  config.vm.provider :virtualbox do |vb|
    vb.memory = {{.Properties.Memory}}
    vb.cpus = {{.Properties.CPUs}}
  end
end`
