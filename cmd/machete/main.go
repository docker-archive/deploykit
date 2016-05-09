package main

import (
	"fmt"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/command"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/docker/libmachete/provisioners/aws"
	"github.com/spf13/cobra"
	"os"
	"os/user"
	"path"
)

func initTemplatesDir(path string) error {
	pathInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(path, 0700)
			if err == nil {
				return nil
			}
		}
		return fmt.Errorf("Failed to initialize templates dir: %s", err)
	}
	if !pathInfo.IsDir() {
		return fmt.Errorf("Template path is not a directory: %s", path)
	}
	return nil
}

func initTemplatesRepo() (libmachete.Templates, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Failed to look up current user: %s", err)
	}

	templatesDir := path.Join(usr.HomeDir, ".machete")
	err = initTemplatesDir(templatesDir)
	if err != nil {
		return nil, err
	}

	templates, err := libmachete.FileTemplates(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to locate templates: %s", err)
	}
	return templates, nil
}

func main() {
	// NOTE: Since credentials are managed externally, we may need a notion of a provisioner
	// that is 'misconfigured' and unable to operate.
	registry := provisioners.NewRegistry(
		map[string]provisioners.ProvisionerBuilder{
			"aws": aws.Builder{},
		})

	output := console.New()
	templates, err := initTemplatesRepo()
	if err != nil {
		output.Fatal(err)
	}

	RootCmd := &cobra.Command{
		Use:   "machete",
		Short: "provision and manage Docker machines across multiple cloud providers"}

	RootCmd.AddCommand(command.ServerCmd())
	RootCmd.AddCommand(command.GetSubcommands(output, registry, templates)...)

	switch err := RootCmd.Execute().(type) {
	case command.HandledError:
		err.Handle()
	case nil:
		// no-op
	default:
		output.Fatal(err)
	}
}
