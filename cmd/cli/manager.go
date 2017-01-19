package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	group_plugin "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func managerCommand(plugins func() discovery.Plugins) *cobra.Command {

	var groupPlugin group.Plugin

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Access the manager",
	}

	commit := cobra.Command{
		Use:   "commit <template_URL>",
		Short: "commit a multi-group configuration, as specified by the URL",
	}
	pretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")
	commit.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", groupPlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		templateURL := args[0]

		log.Infof("Using %v for reading template\n", templateURL)
		engine, err := template.NewTemplate(templateURL, template.Options{
			SocketDir: discovery.Dir(),
		})
		if err != nil {
			return err
		}
		view, err := engine.Render(nil)
		if err != nil {
			return err
		}

		fmt.Print(view)

		// Treat this as an Any and then convert
		any := types.AnyString(view)

		groups := []plugin.Spec{}
		err = any.Decode(&groups)
		if err != nil {
			log.Warningln("Error parsing the template for plugin specs.")
			return err
		}

		// Check the list of plugins
		for _, gp := range groups {
			endpoint, err := plugins().Find(gp.Plugin)
			if err != nil {
				return err
			}

			// unmarshal the group spec
			spec := group.Spec{}
			if gp.Properties != nil {
				err = gp.Properties.Decode(&spec)
				if err != nil {
					return err
				}
			}

			// TODO(chungers) -- we need to enforce and confirm the type of this.
			// Right now we assume the RPC endpoint is indeed a group.
			target := group_plugin.NewClient(endpoint.Address)

			plan, err := target.CommitGroup(spec, *pretend)
			if err != nil {
				return err
			}

			fmt.Println("Group", spec.ID, "with plugin", gp.Plugin, "plan:", plan)
		}

		return nil
	}
	cmd.AddCommand(&commit)

	return cmd
}
