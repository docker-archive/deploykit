package main

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	"github.com/docker/infrakit/pkg/manager"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/spf13/cobra"
)

func init() {
	cli.RegisterInfo("manager - swarm option",
		map[string]interface{}{
			"DockerClientAPIVersion": docker.ClientVersion,
		})
}

type config struct {
	id         string
	plugins    discovery.Plugins
	leader     leader.Detector
	snapshot   store.Snapshot
	pluginName string //This is the name of the stateless group plugin that the manager will proxy for.
}

func main() {

	cmd := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "Manager",
	}

	logLevel := cmd.PersistentFlags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	pluginName := cmd.PersistentFlags().String("name", "group", "Name of the manager")
	backendPlugin := cmd.PersistentFlags().String(
		"proxy-for-group",
		"group-stateless",
		"Name of the group plugin to proxy for.")
	cmd.PersistentPreRun = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)
	}

	buildConfig := func() config {
		return config{
			id:         *pluginName,
			pluginName: *backendPlugin,
		}
	}

	cmd.AddCommand(cli.VersionCommand(), osEnvironment(buildConfig), swarmEnvironment(buildConfig))

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func runMain(cfg config) error {

	log.Infoln("Starting up manager:", cfg)

	mgr, err := manager.NewManager(cfg.plugins, cfg.leader, cfg.snapshot, cfg.pluginName)
	if err != nil {
		return err
	}

	_, err = mgr.Start()
	if err != nil {
		return err
	}

	cli.RunPlugin(cfg.id, group_rpc.PluginServer(mgr))

	mgr.Stop()
	log.Infoln("Manager stopped")

	return err
}
