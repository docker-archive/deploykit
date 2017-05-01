package resource

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	resource_plugin "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/resource")

func init() {
	base.Register(Command)
}

// Command is the top level command
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var resourcePlugin resource.Plugin

	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Access resource plugin",
	}

	name := cmd.PersistentFlags().String("name", "resource", "Name of plugin")
	pretend := cmd.PersistentFlags().Bool("pretend", false, "Don't actually do changes. Explain only where appropriate")

	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
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

		cli.MustNotNil(resourcePlugin, "no resource plugin", "name", *name)
		return nil
	}

	templateFlags, toJSON, fromJSON, processTemplate := base.TemplateProcessor(plugins)

	///////////////////////////////////////////////////////////////////////////////////
	// commit
	commit := &cobra.Command{
		Use:   "commit <template URL>",
		Short: "Commit a resource configuration at url.  Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
				toJSON,
			)
			if err != nil {
				return err
			}

			spec := resource.Spec{}
			if err := types.AnyString(view).Decode(&spec); err != nil {
				return err
			}

			details, err := resourcePlugin.Commit(spec, *pretend)
			if err == nil {
				if *pretend {
					fmt.Printf("Committing %s would involve:\n%s\n", spec.ID, details)
				} else {
					fmt.Printf("Committed %s:\n%s\n", spec.ID, details)
				}
			}
			return err
		},
	}
	commit.Flags().AddFlagSet(templateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// destroy
	destroy := &cobra.Command{
		Use:   "destroy <template URL>",
		Short: "Destroy a resource configuration specified by the URL. Read from stdin if url is '-'",
	}
	destroy.Flags().AddFlagSet(templateFlags)
	destroy.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		view, err := base.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return processTemplate(args[0]) },
			toJSON,
		)
		if err != nil {
			return err
		}

		spec := resource.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		details, err := resourcePlugin.Destroy(spec, *pretend)
		if err == nil {
			if *pretend {
				fmt.Printf("Destroying %s would involve:\n%s\n", spec.ID, details)
			} else {
				fmt.Printf("Destroyed %s:\n%s\n", spec.ID, details)
			}
		}
		return err
	}

	///////////////////////////////////////////////////////////////////////////////////
	// describe
	describe := &cobra.Command{
		Use:   "describe <template URL>",
		Short: "Describe a resource configuration specified by the URL. Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
				toJSON,
			)
			if err != nil {
				return err
			}

			spec := resource.Spec{}
			if err := types.AnyString(view).Decode(&spec); err != nil {
				return err
			}

			details, err := resourcePlugin.DescribeResources(spec)
			if err == nil {
				if len(details) > 0 {

					out, err := fromJSON([]byte(details))
					if err != nil {
						return err
					}

					fmt.Println(string(out))
				}
			}
			return err
		},
	}
	describe.Flags().AddFlagSet(templateFlags)

	cmd.AddCommand(
		commit,
		destroy,
		describe,
	)

	return cmd
}
