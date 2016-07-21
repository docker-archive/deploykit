package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/client"
	"github.com/docker/libmachete/spi/instance"
	"github.com/spf13/cobra"
	"os"
)

// This is mostly a testing utility to validate SSH behavior in real environments.  It is not part of the supported
// machete tools.
func main() {
	rootCmd := &cobra.Command{
		Use: "machete_ssh <machete address> <instance id> <shell code>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 3 {
				cmd.Usage()
				return
			}

			macheteAddress := args[0]
			instanceID := instance.ID(args[1])
			shellCode := args[2]

			output, err := client.NewInstanceProvisioner(macheteAddress).ShellExec(instanceID, shellCode)
			if output != nil {
				fmt.Print(*output)
			}

			if err != nil {
				if err.Error() != "" {
					log.Error(err)
				}

				os.Exit(1)
			}
		},
	}

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
