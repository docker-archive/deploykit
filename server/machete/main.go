package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/server"
	"github.com/spf13/cobra"
	"os"
)

func main() {
	var port uint

	rootCmd := &cobra.Command{Use: "machete"}

	rootCmd.Flags().UintVar(&port, "port", 8888, "Port the server listens on")

	builders := map[string]server.ProvisionerBuilder{
		"aws": &aws.Builder{},
	}

	run := func(cmd *cobra.Command, args []string) {
		log.Infoln("Starting server on port", port)

		provisioner, err := builders[cmd.Name()].BuildInstanceProvisioner()
		if err != nil {
			log.Error(err)
			return
		}

		err = server.RunServer(port, provisioner)
		if err != nil {
			log.Error(err)
		}
	}

	// A subcommand is registered for each provisioner type.  This has no value other than taking advantage of
	// cobra's CLI utility.
	// TODO(wfarner): Override the default usage template to eliminate confusing help output mentioning 'commands'.
	for name, builder := range builders {
		cmd := cobra.Command{Use: name, Run: run}
		cmd.Flags().AddFlagSet(builder.Flags())
		rootCmd.AddCommand(&cmd)
	}

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
