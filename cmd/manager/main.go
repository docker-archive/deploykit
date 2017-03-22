package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	"github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
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

	// Log setup
	logOptions := &log.ProdDefaults

	cmd.PersistentPreRun = func(c *cobra.Command, args []string) {
		log.Configure(logOptions)
	}
	cmd.PersistentFlags().AddFlagSet(cli.Flags(logOptions))
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	pluginName := cmd.PersistentFlags().String("name", "group", "Name of the manager")
	backendPlugin := cmd.PersistentFlags().String(
		"proxy-for-group",
		"group-stateless",
		"Name of the group plugin to proxy for.")

	buildConfig := func() config {
		return config{
			id:         *pluginName,
			pluginName: *backendPlugin,
		}
	}

	cmd.AddCommand(cli.VersionCommand(),
		osEnvironment(buildConfig),
		swarmEnvironment(buildConfig),
		etcdEnvironment(buildConfig),
	)

	err := cmd.Execute()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

func runMain(cfg config) error {

	logrus.Infoln("Starting up manager:", cfg)

	mgr, err := manager.NewManager(cfg.plugins, cfg.leader, cfg.snapshot, cfg.pluginName)
	if err != nil {
		return err
	}

	_, err = mgr.Start()
	if err != nil {
		return err
	}

	// Start a poller to load the snapshot and make that available as metadata
	updateSnapshot := make(chan func(map[string]interface{}))
	stopSnapshot := make(chan struct{})
	go func() {
		tick := time.Tick(1 * time.Second)
		for {
			select {
			case <-tick:
				snapshot := map[string]interface{}{}

				// update leadership
				if isLeader, err := mgr.IsLeader(); err == nil {
					updateSnapshot <- func(view map[string]interface{}) {
						metadata_plugin.Put([]string{"leader"}, isLeader, view)
					}
				} else {
					logrus.Warningln("Cannot check leader for metadata:", err)
				}

				// update config
				if err := cfg.snapshot.Load(&snapshot); err == nil {
					updateSnapshot <- func(view map[string]interface{}) {
						metadata_plugin.Put([]string{"configs"}, snapshot, view)
					}
				} else {
					logrus.Warningln("Cannot load snapshot for metadata:", err)
				}

			case <-stopSnapshot:
				logrus.Infoln("Snapshot updater stopped")
				return
			}
		}
	}()

	cli.RunPlugin(cfg.id,
		metadata_rpc.PluginServer(metadata_plugin.NewPluginFromChannel(updateSnapshot)),
		group_rpc.PluginServer(mgr), manager_rpc.PluginServer(mgr))

	mgr.Stop()
	close(stopSnapshot)
	logrus.Infoln("Manager stopped")

	return err
}
