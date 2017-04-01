package resource

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	resource_plugin "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
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

	commit := cobra.Command{
		Use:   "commit <template URL>",
		Short: "commit a resource configuration specified by the URL",
	}
	commitPretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")
	commitTemplateFlags, commitProcessTemplate := base.TemplateProcessor(plugins)
	commit.Flags().AddFlagSet(commitTemplateFlags)

	commit.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		view, err := commitProcessTemplate(args[0])
		if err != nil {
			return err
		}

		spec := resource.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		details, err := resourcePlugin.Commit(spec, *commitPretend)
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
	destroyTemplateFlags, destroyProcessTemplate := base.TemplateProcessor(plugins)
	destroy.Flags().AddFlagSet(destroyTemplateFlags)
	destroy.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		view, err := destroyProcessTemplate(args[0])
		if err != nil {
			return err
		}

		spec := resource.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		details, err := resourcePlugin.Destroy(spec, *destroyPretend)
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
	describeTemplateFlags, describeProcessTemplate := base.TemplateProcessor(plugins)
	describe.Flags().AddFlagSet(describeTemplateFlags)
	describe.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		view, err := describeProcessTemplate(args[0])
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
				fmt.Println(details)
			}
		}
		return err
	}
	cmd.AddCommand(&describe)

	return cmd
}

func _readSpecFromTemplateURL(templateURL string, globals []string) (*resource.Spec, error) {
	log.Info("Reading template", "url", templateURL)
	engine, err := template.NewTemplate(templateURL, template.Options{})
	if err != nil {
		return nil, err
	}

	for _, global := range globals {
		kv := strings.SplitN(global, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key != "" && val != "" {
			engine.Global(key, val)
		}
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

	log.Debug("rendered", "view", view)

	spec := resource.Spec{}
	if err := types.AnyString(view).Decode(&spec); err != nil {
		log.Warn("Error parsing template", "err", err)
		return nil, err
	}

	return &spec, nil
}
