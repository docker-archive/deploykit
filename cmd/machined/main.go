package main

import (
	"github.com/docker/libmachete/cmd/machined/command"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "machined",
		Short: "machine daemon",
	}

	rootCmd.AddCommand(command.ServerCmd())

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
