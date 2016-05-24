package main

import (
	"github.com/docker/libmachete/cmd/machined/http"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "machined",
		Short: "machine daemon",
	}

	rootCmd.AddCommand(http.ServerCmd(), http.ClientCmd())

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
