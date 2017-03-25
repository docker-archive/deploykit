package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	resource_plugin "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func resourcePluginCommand(plugins func() discovery.Plugins) *cobra.Command {

	var resourcePlugin resource.Plugin

	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Access resource plugin",
	}
	name := cmd.PersistentFlags().String("name", "resource", "Name of plugin")
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := upTree(c, func(x *cobra.Command, argv []string) error {
			if x.PersistentPreRunE != nil {
				return x.PersistentPreRunE(x, argv)
			}
			return nil
		}); err != nil {
			return err
		}

		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		p, err := resource_plugin.NewClient(endpoint.Address)
		if err != nil {
			return err
		}
		resourcePlugin = p
		return nil
	}

	commit := cobra.Command{
		Use:   "commit <template URL>",
		Short: "commit a resource configuration specified by the URL",
	}
	commitPretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")
	commit.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		spec, err := readSpecFromTemplateURL(args[0])
		if err != nil {
			return err
		}

		details, err := resourcePlugin.Commit(*spec, *commitPretend)
		if err == nil {
			if *commitPretend {
				fmt.Printf("Committing %s would involve:\n%s\n", spec.ID, details)
			} else {
				fmt.Printf("Committed %s:\n%s\n", spec.ID, details)
			}
		}
		return err
	}
	cmd.AddCommand(&commit)

	destroy := cobra.Command{
		Use:   "destroy <template URL>",
		Short: "destroy a resource configuration specified by the URL",
	}
	destroyPretend := destroy.Flags().Bool("pretend", false, "Don't actually destroy, only explain the destroy")
	destroy.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		spec, err := readSpecFromTemplateURL(args[0])
		if err != nil {
			return err
		}

		details, err := resourcePlugin.Destroy(*spec, *destroyPretend)
		if err == nil {
			if *destroyPretend {
				fmt.Printf("Destroying %s would involve:\n%s\n", spec.ID, details)
			} else {
				fmt.Printf("Destroyed %s:\n%s\n", spec.ID, details)
			}
		}
		return err
	}
	cmd.AddCommand(&destroy)

	describe := cobra.Command{
		Use:   "describe <template URL>",
		Short: "describe a resource configuration specified by the URL",
	}
	describe.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		spec, err := readSpecFromTemplateURL(args[0])
		if err != nil {
			return err
		}

		details, err := resourcePlugin.DescribeResources(*spec)
		if err == nil {
			if len(details) > 0 {
				fmt.Println(details)
			}
		}
		return err
	}
	cmd.AddCommand(&describe)

	return cmd
}

func readSpecFromTemplateURL(templateURL string) (*resource.Spec, error) {
	log.Infof("Reading template from %v", templateURL)
	engine, err := template.NewTemplate(templateURL, template.Options{
		SocketDir: local.Dir(),
	})
	if err != nil {
		return nil, err
	}

	engine.WithFunctions(func() []template.Function {
		return []template.Function{
			{Name: "resource", Func: func(s string) string { return fmt.Sprintf("{{ resource `%s` }}", s) }},
		}
	})

	view, err := engine.Render(nil)
	if err != nil {
		return nil, err
	}

	log.Debugln(view)

	spec := resource.Spec{}
	if err := types.AnyString(view).Decode(&spec); err != nil {
		log.Warningln("Error parsing template")
		return nil, err
	}

	return &spec, nil
}
