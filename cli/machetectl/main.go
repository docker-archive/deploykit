package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/aws/awsbootstrap"
	"github.com/docker/libmachete/spi/cli"
	"github.com/spf13/cobra"
	"os"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func attachDriver(rootCmd *cobra.Command, cli cli.DriverCli, requiredName string) {
	cmd := cli.Command()
	if cmd.Name() != requiredName {
		panic(fmt.Sprintf("Internal error - driver must use name '%s'", requiredName))
	}

	rootCmd.AddCommand(cmd)
}

func main() {
	rootCmd := &cobra.Command{Use: "machetecli"}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	attachDriver(rootCmd, awsbootstrap.NewDriverCli(), "aws")

	err := rootCmd.Execute()
	if err != nil {
		log.Print(err)
		os.Exit(-1)
	}
}
