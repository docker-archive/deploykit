package init

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	flavor_rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	run "github.com/docker/infrakit/pkg/run/manager"
	group_kind "github.com/docker/infrakit/pkg/run/v0/group"
	manager_kind "github.com/docker/infrakit/pkg/run/v0/manager"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "util/init")

func startPlugins(plugins func() discovery.Plugins, services *cli.Services,
	configURL string, starts []string) (*run.Manager, error) {

	parsedRules := []launch.Rule{}

	if configURL != "" {
		buff, err := services.ProcessTemplate(configURL)
		if err != nil {
			return nil, err
		}
		view, err := services.ToJSON([]byte(buff))
		if err != nil {
			return nil, err
		}
		configs := types.AnyBytes(view)
		err = configs.Decode(&parsedRules)
		if err != nil {
			return nil, err
		}
	}

	pluginManager, err := run.ManagePlugins(parsedRules, plugins, true, 5*time.Second)
	if err != nil {
		return nil, err
	}

	for _, arg := range starts {

		p := strings.Split(arg, "=")
		execName := "inproc" // default is to use inprocess goroutine for running plugins
		if len(p) > 1 {
			execName = p[1]
		}

		// the format is kind[:{plugin_name}][={os|inproc}]
		pp := strings.Split(p[0], ":")
		kind := pp[0]
		name := plugin.Name(kind)

		// This is some special case for the legacy setup (pre v0.6)
		switch kind {
		case manager_kind.Kind:
			name = plugin.Name(manager_kind.LookupName)
		case group_kind.Kind:
			name = plugin.Name(group_kind.LookupName)
		}
		// customized by user as override
		if len(pp) > 1 {
			name = plugin.Name(pp[1])
		}

		log.Info("Launching", "kind", kind, "name", name)
		err = pluginManager.Launch(execName, kind, name, nil)
		if err != nil {
			log.Warn("failed to launch", "exec", execName, "kind", kind, "name", name)
		}
	}
	return pluginManager, nil
}

// Command returns the cobra command
func Command(plugins func() discovery.Plugins) *cobra.Command {

	services := cli.NewServices(plugins)

	cmd := &cobra.Command{
		Use:   "init <groups template URL | - >",
		Short: "Generates the init script",
	}

	cmd.Flags().AddFlagSet(services.ProcessTemplateFlags)
	groupID := cmd.Flags().String("group-id", "", "Group ID")

	configURL := cmd.Flags().String("config-url", "", "URL for the startup configs")
	starts := cmd.Flags().StringSlice("start", []string{}, "start spec for plugin just like infrakit plugin start")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		logutil.Configure(&logutil.Options{
			Level:    3,
			Stdout:   false,
			Format:   "term",
			CallFunc: true,
		})

		pluginManager, err := startPlugins(plugins, services, *configURL, *starts)
		if err != nil {
			return err
		}
		defer func() {
			pluginManager.TerminateAll()
			pluginManager.WaitForAllShutdown()
			pluginManager.Stop()
		}()

		pluginManager.WaitStarting()

		input, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		// TODO - update the schema -- this matches the current Plugin/Properties schema
		type spec struct {
			Plugin     plugin.Name
			Properties struct {
				ID         group.ID
				Properties group_types.Spec
			}
		}

		specs := []spec{}
		err = types.AnyString(input).Decode(&specs)
		if err != nil {
			return err
		}

		var groupSpec *group_types.Spec
		for _, s := range specs {
			if string(s.Properties.ID) == *groupID {
				copy := s.Properties.Properties
				groupSpec = &copy
				break
			}
		}

		if groupSpec == nil {
			return fmt.Errorf("no such group: %v", *groupID)
		}

		// Get the flavor properties and use that to call the prepare of the Flavor to generate the init
		endpoint, err := plugins().Find(groupSpec.Flavor.Plugin)
		if err != nil {
			return err
		}

		flavorPlugin, err := flavor_rpc.NewClient(groupSpec.Flavor.Plugin, endpoint.Address)
		if err != nil {
			return err
		}

		cli.MustNotNil(flavorPlugin, "flavor plugin not found", "name", groupSpec.Flavor.Plugin.String())

		instanceSpec := instance.Spec{}
		if len(groupSpec.Allocation.LogicalIDs) > 0 {
			lid := instance.LogicalID(groupSpec.Allocation.LogicalIDs[0])
			instanceSpec.LogicalID = &lid
		}

		instanceSpec, err = flavorPlugin.Prepare(groupSpec.Flavor.Properties, instanceSpec,
			groupSpec.Allocation,
			group_types.Index{Group: group.ID(*groupID), Sequence: 0})

		if err != nil {
			return err
		}

		fmt.Print(instanceSpec.Init)
		return nil
	}

	return cmd
}
