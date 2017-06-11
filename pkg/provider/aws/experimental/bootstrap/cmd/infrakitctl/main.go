package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.aws/experimental/bootstrap"
	"github.com/spf13/cobra"
	"os"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {
	rootCmd := &cobra.Command{Use: "infrakitcli"}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	bootstrap.NewCLI().AddCommands(rootCmd)

	err := rootCmd.Execute()
	if err != nil {
		log.Print(err)
		os.Exit(-1)
	}
}
