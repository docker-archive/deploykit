package main

import (
	"errors"
	"fmt"
	"github.com/docker/libmachete/machete/cmd"
	"github.com/docker/libmachete/provisioners"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/spf13/cobra"
	"os"
)

type awsCreator struct {
}

func (a awsCreator) Create(params map[string]string) (api.Provisioner, error) {
	return nil, errors.New("not implemented")
}

func main() {
	// NOTE: Since credentials are managed externally, we may need a notion of a provisioner
	// that is 'misconfigured' and unable to operate.
	registry := provisioners.NewRegistry(
		map[string]provisioners.Creator{
			"aws": awsCreator{},
		})

	RootCmd := &cobra.Command{
		Use:   "machete",
		Short: "provision and manage Docker machines across multiple cloud providers"}

	RootCmd.AddCommand(cmd.GetSubcommands(registry)...)

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
