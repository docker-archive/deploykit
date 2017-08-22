package x

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/controller/ingress"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/rpc/client"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func ingressCommand(plugins func() discovery.Plugins) *cobra.Command {

	cliServices := cli.NewServices(plugins)

	cmd := &cobra.Command{
		Use:   "ingress <url | - >...",
		Short: "Starts the ingress controller",
	}
	groupPluginName := cmd.Flags().String("group", "group", "Group plugin name")
	cmd.Flags().AddFlagSet(cliServices.ProcessTemplateFlags)
	cmd.RunE = func(c *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(-1)
		}

		view, err := cliServices.ReadFromStdinOrURL(args[0])
		if err != nil {
			return err
		}

		done, err := handleConfigAndRun(plugins, *groupPluginName, types.AnyString(view))
		if err != nil {
			return err
		}

		return <-done
	}
	return cmd
}

func leadership(plugins func() discovery.Plugins) (manager.Leadership, error) {
	// Scan for a manager
	pm, err := plugins().List()
	if err != nil {
		return nil, err
	}

	for _, endpoint := range pm {
		rpcClient, err := client.New(endpoint.Address, manager.InterfaceSpec)
		if err == nil {
			return manager_rpc.Adapt(rpcClient), nil
		}
	}
	return nil, nil
}

func handleConfigAndRun(plugins func() discovery.Plugins, groupPluginName string, blob *types.Any) (<-chan error, error) {
	yaml, err := blob.MarshalYAML()
	if err != nil {
		return nil, err
	}
	log.Info(string(yaml))

	leadership, err := leadership(plugins)
	if err != nil {
		return nil, err
	}

	c := ingress.NewController(leadership)

	spec := types.Spec{}
	err = blob.Decode(&spec)
	if err != nil {
		return nil, err
	}

	log.Info("Starting controller")

	current, plan, err := c.Plan(controller.Manage, spec)

	log.Info("Plan", "object", current, "plan", plan, "err", err)
	if err != nil {
		return nil, err
	}

	errChan := make(chan error, 1)

	current, err = c.Commit(controller.Manage, spec)

	errChan <- err
	close(errChan)

	if err != nil {
		return nil, err
	}

	return errChan, err
}
